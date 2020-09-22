package nodelease

// Most code borrowed from: pkg/kubelet/nodelease/controller.go
import (
	"context"
	"fmt"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	coordclientset "k8s.io/client-go/kubernetes/typed/coordination/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
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

// Controller manages creating and renewing the lease for this Kubelet
type Controller interface {
	Run(context context.Context)
}

type controller struct {
	nodeClient                 corev1client.NodeInterface
	leaseClient                coordclientset.LeaseInterface
	holderIdentity             string
	leaseDuration              time.Duration
	renewInterval              time.Duration
	clock                      clock.Clock
	onRepeatedHeartbeatFailure func()

	// latestLease is the latest node lease which Kubelet updated or created
	latestLease *coordinationv1.Lease
}

// NewController creates a new controller.
func NewController(clock clock.Clock, nodeClient corev1client.NodeInterface, leaseClient coordclientset.LeaseInterface, holderIdentity string, leaseDuration time.Duration, onRepeatedHeartbeatFailure func()) Controller {
	return &controller{
		nodeClient:                 nodeClient,
		leaseClient:                leaseClient,
		holderIdentity:             holderIdentity,
		leaseDuration:              leaseDuration,
		renewInterval:              time.Duration(float64(leaseDuration) * renewIntervalFraction),
		clock:                      clock,
		onRepeatedHeartbeatFailure: onRepeatedHeartbeatFailure,
	}
}

// Run runs the controller
func (c *controller) Run(ctx context.Context) {
	wait.UntilWithContext(ctx, c.sync, c.renewInterval)
}

func (c *controller) sync(ctx context.Context) {
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
			return
		}
		log.G(ctx).WithError(err).Info("failed to update lease using latest lease, fallback to ensure lease")
	}

	lease, created := c.backoffEnsureLease(ctx)
	c.latestLease = lease
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
func (c *controller) backoffEnsureLease(ctx context.Context) (*coordinationv1.Lease, bool) {
	var (
		lease   *coordinationv1.Lease
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
		log.G(ctx).WithError(err).Errorf("failed to ensure node lease exists, will retry in %v", sleep)
		// backoff wait
		c.clock.Sleep(sleep)
	}
	return lease, created
}

// ensureLease creates the lease if it does not exist. Returns the lease and
// a bool (true if this call created the lease), or any error that occurs.
func (c *controller) ensureLease(ctx context.Context) (*coordinationv1.Lease, bool, error) {
	lease, err := c.leaseClient.Get(context.TODO(), c.holderIdentity, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// lease does not exist, create it.
		leaseToCreate := c.newLease(ctx, nil)
		if len(leaseToCreate.OwnerReferences) == 0 {
			// We want to ensure that a lease will always have OwnerReferences set.
			// Thus, given that we weren't able to set it correctly, we simply
			// not create it this time - we will retry in the next iteration.
			return nil, false, nil
		}
		lease, err := c.leaseClient.Create(context.TODO(), leaseToCreate, metav1.CreateOptions{})
		if err != nil {
			return nil, false, err
		}
		return lease, true, nil
	} else if err != nil {
		// unexpected error getting lease
		return nil, false, err
	}
	// lease already existed
	return lease, false, nil
}

// retryUpdateLease attempts to update the lease for maxUpdateRetries,
// call this once you're sure the lease has been created
func (c *controller) retryUpdateLease(ctx context.Context, base *coordinationv1.Lease) error {
	for i := 0; i < maxUpdateRetries; i++ {
		lease, err := c.leaseClient.Update(ctx, c.newLease(ctx, base), metav1.UpdateOptions{})
		if err == nil {
			c.latestLease = lease
			return nil
		}
		log.G(ctx).WithError(err).Error("failed to update node lease")
		// OptimisticLockError requires getting the newer version of lease to proceed.
		if apierrors.IsConflict(err) {
			base, _ = c.backoffEnsureLease(ctx)
			continue
		}
		if i > 0 && c.onRepeatedHeartbeatFailure != nil {
			c.onRepeatedHeartbeatFailure()
		}
	}
	return fmt.Errorf("failed %d attempts to update node lease", maxUpdateRetries)
}

// newLease constructs a new lease if base is nil, or returns a copy of base
// with desired state asserted on the copy.
func (c *controller) newLease(ctx context.Context, base *coordinationv1.Lease) *coordinationv1.Lease {
	// Use the bare minimum set of fields; other fields exist for debugging/legacy,
	// but we don't need to make node heartbeats more complicated by using them.
	var lease *coordinationv1.Lease
	if base == nil {
		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      c.holderIdentity,
				Namespace: corev1.NamespaceNodeLease,
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       pointer.StringPtr(c.holderIdentity),
				LeaseDurationSeconds: pointer.Int32Ptr(int32(c.leaseDuration.Seconds())),
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
		if node, err := c.nodeClient.Get(context.TODO(), c.holderIdentity, metav1.GetOptions{}); err == nil {
			lease.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
					Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
					Name:       c.holderIdentity,
					UID:        node.UID,
				},
			}
		} else {
			log.G(ctx).WithError(err).Errorf("failed to get node %q when trying to set owner ref to the node lease", c.holderIdentity)
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
