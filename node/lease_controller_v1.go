package node

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
import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	coordclientset "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
)

const (
	// renewIntervalFraction is the fraction of lease duration to renew the lease
	renewIntervalFraction = 0.25
	// maxUpdateRetries is the number of immediate, successive retries the Kubelet will attempt
	// when renewing the lease before it waits for the renewal interval before trying again,
	// similar to what we do for node status retries
	maxUpdateRetries = 5
	// maxBackoff is the maximum sleep time during backoff (e.g. in backoffEnsureLease)
	maxBackoff = 7 * time.Second
)

type controller struct {
	leaseClient          coordclientset.LeaseInterface
	leaseDurationSeconds int32
	renewInterval        time.Duration
	clock                clock.Clock
	pingStatusFunc       func(context.Context) (*pingResult, error)
	getNodeFunc          func(context.Context) (*corev1.Node, error)

	// latestLease is the latest node lease which Kubelet updated or created
	latestLease *coordinationv1.Lease
}

// newController constructs and returns a controller
func newController(
	clock clock.Clock,
	client coordclientset.LeaseInterface,
	leaseDurationSeconds int32,
	pingStatusFunc func(context.Context) (*pingResult, error),
	getNodeFunc func(context.Context) (*corev1.Node, error)) leaseController {

	leaseDuration := time.Duration(leaseDurationSeconds) * time.Second
	renewInterval := time.Duration(float64(leaseDuration) * renewIntervalFraction)
	return newControllerWithRenewInterval(
		clock,
		client,
		leaseDurationSeconds,
		renewInterval,
		pingStatusFunc,
		getNodeFunc)
}

// newControllerWithRenewInterval constructs and returns a v1 lease controller
func newControllerWithRenewInterval(
	clock clock.Clock,
	client coordclientset.LeaseInterface,
	leaseDurationSeconds int32,
	renewInterval time.Duration,
	pingStatusFunc func(context.Context) (*pingResult, error),
	getNodeFunc func(context.Context) (*corev1.Node, error)) leaseController {

	return &controller{
		leaseClient:          client,
		leaseDurationSeconds: leaseDurationSeconds,
		renewInterval:        renewInterval,
		clock:                clock,
		pingStatusFunc:       pingStatusFunc,
		getNodeFunc:          getNodeFunc,
	}
}

// Run runs the controller
func (c *controller) run(ctx context.Context) {
	if c.leaseClient == nil {
		log.G(ctx).Infof("node lease controller has nil lease client, will not claim or renew leases")
		return
	}
	c.sync(ctx)
	wait.UntilWithContext(ctx, c.sync, c.renewInterval)
}

func (c *controller) sync(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var err error
	ctx, span := trace.StartSpan(ctx, "controller.sync")
	defer span.End()

	pingResult, err := c.pingStatusFunc(ctx)
	if err != nil {
		log.G(ctx).WithError(err).Error("Could not get ping status")
	}
	if pingResult.error != nil {
		log.G(ctx).WithError(pingResult.error).Error("Ping result is not clean, not updating lease")
	}

	node, err := c.getNodeFunc(ctx)
	if err != nil {
		log.G(ctx).WithError(err).Error("Could not get server node")
		span.SetStatus(err)
		return
	}
	if node == nil {
		err = errors.New("Servernode is null")
		log.G(ctx).WithError(err).Error("servernode is null")
		span.SetStatus(err)
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
		err := c.retryUpdateLease(ctx, node, c.newLease(ctx, node, c.latestLease))
		if err == nil {
			span.SetStatus(err)
			return
		}
		log.G(ctx).WithError(err).Info("failed to update lease using latest lease, fallback to ensure lease")
	}

	lease, created := c.backoffEnsureLease(ctx, node)
	c.latestLease = lease
	// we don't need to update the lease if we just created it
	if !created && lease != nil {
		if err := c.retryUpdateLease(ctx, node, lease); err != nil {
			log.G(ctx).WithError(err).WithField("renewInterval", c.renewInterval).Errorf("Will retry after")
			span.SetStatus(err)
		}
	}
}

// backoffEnsureLease attempts to create the lease if it does not exist,
// and uses exponentially increasing waits to prevent overloading the API server
// with retries. Returns the lease, and true if this call created the lease,
// false otherwise.
func (c *controller) backoffEnsureLease(ctx context.Context, node *corev1.Node) (*coordinationv1.Lease, bool) {
	ctx, span := trace.StartSpan(ctx, "controller.backoffEnsureLease")
	defer span.End()

	var (
		lease   *coordinationv1.Lease
		created bool
		err     error
	)
	sleep := 100 * time.Millisecond
	for {
		lease, created, err = c.ensureLease(ctx, node)
		if err == nil {
			break
		}
		sleep = minDuration(2*sleep, maxBackoff)
		log.G(ctx).WithError(err).Errorf("failed to ensure node lease exists, will retry in %v", sleep)
		// backoff wait
		c.clock.Sleep(sleep)
		timer := c.clock.NewTimer(sleep)
		defer timer.Stop()
		select {
		case <-timer.C():
		case <-ctx.Done():
			return nil, false
		}
	}
	return lease, created
}

// ensureLease creates the lease if it does not exist. Returns the lease and
// a bool (true if this call created the lease), or any error that occurs.
func (c *controller) ensureLease(ctx context.Context, node *corev1.Node) (*coordinationv1.Lease, bool, error) {
	ctx, span := trace.StartSpan(ctx, "controller.ensureLease")
	defer span.End()

	lease, err := c.leaseClient.Get(ctx, node.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// lease does not exist, create it.
		leaseToCreate := c.newLease(ctx, node, nil)
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
		log.G(ctx).Debug("Successfully created lease")
		return lease, true, nil
	} else if err != nil {
		// unexpected error getting lease
		log.G(ctx).WithError(err).Error("Unexpected error getting lease")
		span.SetStatus(err)
		return nil, false, err
	}
	log.G(ctx).Debug("Successfully recovered existing lease")
	// lease already existed
	return lease, false, nil
}

// retryUpdateLease attempts to update the lease for maxUpdateRetries,
// call this once you're sure the lease has been created
func (c *controller) retryUpdateLease(ctx context.Context, node *corev1.Node, base *coordinationv1.Lease) error {
	ctx, span := trace.StartSpan(ctx, "controller.retryUpdateLease")
	defer span.End()

	for i := 0; i < maxUpdateRetries; i++ {
		lease, err := c.leaseClient.Update(ctx, c.newLease(ctx, node, base), metav1.UpdateOptions{})
		if err == nil {
			log.G(ctx).WithField("retries", i).Debug("Successfully updated lease")
			c.latestLease = lease
			return nil
		}
		log.G(ctx).WithError(err).Error("failed to update node lease")
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			break
		}
		// OptimisticLockError requires getting the newer version of lease to proceed.
		if apierrors.IsConflict(err) {
			base, _ = c.backoffEnsureLease(ctx, node)
			continue
		}
	}
	err := fmt.Errorf("failed %d attempts to update node lease", maxUpdateRetries)
	span.SetStatus(err)
	return err
}

// newLease constructs a new lease if base is nil, or returns a copy of base
// with desired state asserted on the copy.
func (c *controller) newLease(ctx context.Context, node *corev1.Node, base *coordinationv1.Lease) *coordinationv1.Lease {
	ctx, span := trace.StartSpan(ctx, "controller.newLease")
	defer span.End()
	// Use the bare minimum set of fields; other fields exist for debugging/legacy,
	// but we don't need to make node heartbeats more complicated by using them.
	var lease *coordinationv1.Lease
	if base == nil {
		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      node.Name,
				Namespace: corev1.NamespaceNodeLease,
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       pointer.StringPtr(node.Name),
				LeaseDurationSeconds: pointer.Int32Ptr(c.leaseDurationSeconds),
			},
		}
	} else {
		lease = base.DeepCopy()
	}
	lease.Spec.RenewTime = &metav1.MicroTime{Time: c.clock.Now()}

	// Setting owner reference needs node's UID. Note that it is different from
	// kubelet.nodeRef.UID. When lease is initially created, it is possible that
	// the connection between master and node is not ready yet. So try to set
	// owner reference every time when renewing the lease, until successful.
	if len(lease.OwnerReferences) == 0 {
		lease.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
				Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
				Name:       node.Name,
				UID:        node.UID,
			},
		}
	}

	ctx = span.WithFields(ctx, map[string]interface{}{
		"lease": lease,
	})
	log.G(ctx).Debug("Generated lease")
	return lease
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
