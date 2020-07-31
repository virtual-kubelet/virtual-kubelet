/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	coord "k8s.io/api/coordination/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/typed/coordination/v1beta1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/utils/pointer"
)

const (
	// maxUpdateRetries is the number of immediate, successive retries the Kubelet will attempt
	// when renewing the lease before it waits for the renewal interval before trying again,
	// similar to what we do for node status retries
	maxUpdateRetries = 5
	// maxBackoff is the maximum sleep time during backoff (e.g. in backoffEnsureLease)
	maxBackoff = 7 * time.Second

	virtualKubeletCreatedLease = "virtual-kubelet.io/created-lease"
)

type leaseController struct {
	leaseClient v1beta1.LeaseInterface
	// How long the lease should last
	leaseDurationSeconds int32
	// How often we should renew the lease
	renewInterval time.Duration

	baseLease *coord.Lease

	latestLease *coord.Lease

	// The following must be set after pod controller start:
	// Name of node we are responsible for
	holderIdentity    string
	nodesClient       v1.NodeInterface
	firstSyncComplete chan struct{}
	shouldUpdateLease func(context.Context) bool

	leasesWorkingLock sync.Mutex
	leasesWorking     bool
}

// Run runs the leaseController
func (c *leaseController) run(ctx context.Context) {
	// As normal, do the first one by hand.
	c.sync(ctx)
	close(c.firstSyncComplete)
	wait.UntilWithContext(ctx, c.sync, c.renewInterval)
}

func (c *leaseController) getLeasesWorking(ctx context.Context) bool {
	if c == nil {
		return false
	}

	select {
	case <-ctx.Done():
		// This is kind of weird.
		log.G(ctx).WithError(ctx.Err()).Warn("Context completed before first lease sync complete")
		return false
	case <-c.firstSyncComplete:
	}

	c.leasesWorkingLock.Lock()
	defer c.leasesWorkingLock.Unlock()
	return c.leasesWorking
}

func (c *leaseController) sync(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "leaseController.sync")
	defer span.End()

	if shouldUpdateLease := c.shouldUpdateLease(ctx); !shouldUpdateLease {
		log.G(ctx).Debug("Leases disabled, either due node ping not responding or other reason")
		return
	}

	if c.latestLease != nil {
		// As long as node lease is not (or very rarely) updated by any other agent than Kubelet,
		// we can optimistically assume it didn't change since our last update and try updating
		// based on the version from that time. Thanks to it we avoid GET call and reduce load
		// on etcd and kube-apiserver.
		// If at some point other agents will also be frequently updating the Lease object, this
		// can result in performance degradation, because we will end up with calling additional
		// GET/PUT - at this point this whole "if" should be removed.
		err := c.retryUpdateLease(ctx, c.newLease(ctx, c.latestLease))
		if err == nil {
			span.SetStatus(err)
			return
		}
		log.G(ctx).WithError(err).Info("failed to update lease using latest lease, fallback to ensure lease")
	}

	lease, created := c.backoffEnsureLease(ctx)
	c.latestLease = lease
	c.leasesWorkingLock.Lock()
	c.leasesWorking = true
	c.leasesWorkingLock.Unlock()
	// we don't need to update the lease if we just created it
	if !created && lease != nil {
		if err := c.retryUpdateLease(ctx, lease); err != nil {
			log.G(ctx).WithError(err).Error("Will retry after %v", c.renewInterval)
		}
	}
}

// backoffEnsureLease attempts to create the lease if it does not exist,
// and uses exponentially increasing waits to prevent overloading the API server
// with retries. Returns the lease, and true if this call created the lease,
// false otherwise.
func (c *leaseController) backoffEnsureLease(ctx context.Context) (*coord.Lease, bool) {
	var (
		lease   *coord.Lease
		created bool
		err     error
	)
	sleep := 100 * time.Millisecond
	for {
		lease, created, err = c.ensureLease(ctx)
		if err == nil {
			break
		}
		sleep = minDuration(2*sleep, maxBackoff)
		log.G(ctx).WithError(err).Error("failed to ensure node lease exists, will retry in %v", sleep)
		// backoff wait
		time.Sleep(sleep)
	}
	return lease, created
}

// ensureLease creates the lease if it does not exist. Returns the lease and
// a bool (true if this call created the lease), or any error that occurs.
func (c *leaseController) ensureLease(ctx context.Context) (*coord.Lease, bool, error) {
	ctx, span := trace.StartSpan(ctx, "leaseController.ensureLease")
	defer span.End()

	lease, err := c.leaseClient.Get(ctx, c.holderIdentity, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// lease does not exist, create it.
		leaseToCreate := c.newLease(ctx, c.baseLease)
		if len(leaseToCreate.OwnerReferences) == 0 {
			// We want to ensure that a lease will always have OwnerReferences set.
			// Thus, given that we weren't able to set it correctly, we simply
			// not create it this time - we will retry in the next iteration.
			return nil, false, nil
		}
		lease, err := c.leaseClient.Create(ctx, leaseToCreate, metav1.CreateOptions{})
		if err != nil {
			span.SetStatus(err)
			return nil, false, err
		}
		return lease, true, nil
	} else if err != nil {
		// unexpected error getting lease
		span.SetStatus(err)
		return nil, false, err
	}

	// lease already existed. Previous virtual kubelet behaviour was to delete the existing lease, and
	// recreate it in our own image. I don't think that's so viable here. Before we squash it, let's see if
	// we just created it before (say this was a restart?)
	if _, ok := lease.Annotations[virtualKubeletCreatedLease]; ok {
		return lease, false, nil
	}

	log.G(ctx).Warn("Squashing existing lease")
	squashLease := c.newLease(ctx, c.baseLease)
	squashLease.ResourceVersion = ""
	lease, err = c.leaseClient.Update(ctx, squashLease, metav1.UpdateOptions{})
	if err != nil {
		span.SetStatus(err)
		return nil, false, err
	}
	return lease, true, nil
}

// retryUpdateLease attempts to update the lease for maxUpdateRetries,
// call this once you're sure the lease has been created
func (c *leaseController) retryUpdateLease(ctx context.Context, base *coord.Lease) error {
	for i := 0; i < maxUpdateRetries; i++ {
		lease, err := c.leaseClient.Update(ctx, c.newLease(ctx, base), metav1.UpdateOptions{})
		if err == nil {
			c.leasesWorkingLock.Lock()
			c.leasesWorking = true
			c.leasesWorkingLock.Unlock()
			c.latestLease = lease
			return nil
		}
		log.G(ctx).WithError(err).Errorf("failed to update node lease, error")
		// OptimisticLockError requires getting the newer version of lease to proceed.
		if apierrors.IsConflict(err) {
			base, _ = c.backoffEnsureLease(ctx)
			continue
		}
	}
	return fmt.Errorf("failed %d attempts to update node lease", maxUpdateRetries)
}

// newLease constructs a new lease if base is nil, or returns a copy of base
// with desired state asserted on the copy.
func (c *leaseController) newLease(ctx context.Context, base *coord.Lease) *coord.Lease {
	// Use the bare minimum set of fields; other fields exist for debugging/legacy,
	// but we don't need to make node heartbeats more complicated by using them.
	var lease *coord.Lease
	if base == nil {
		lease = &coord.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      c.holderIdentity,
				Namespace: corev1.NamespaceNodeLease,
			},
			Spec: coord.LeaseSpec{
				HolderIdentity:       pointer.StringPtr(c.holderIdentity),
				LeaseDurationSeconds: pointer.Int32Ptr(c.leaseDurationSeconds),
			},
		}
	} else {
		lease = base.DeepCopy()
	}
	lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now()}

	// Setting owner reference needs node's UID. Note that it is different from
	// kubelet.nodeRef.UID. When lease is initially created, it is possible that
	// the connection between master and node is not ready yet. So try to set
	// owner reference every time when renewing the lease, until successful.
	if len(lease.OwnerReferences) == 0 {
		if node, err := c.nodesClient.Get(ctx, c.holderIdentity, metav1.GetOptions{}); err == nil {
			lease.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
					Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
					Name:       c.holderIdentity,
					UID:        node.UID,
				},
			}
		} else {
			log.G(ctx).WithError(err).Error("failed to get node %q when trying to set owner ref to the node lease", c.holderIdentity)
		}
	}

	return lease
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
