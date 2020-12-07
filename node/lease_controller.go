package node

import (
	"context"
	pkgerrors "errors"
	"fmt"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	coord "k8s.io/api/coordination/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/coordination/v1beta1"
)

type leaseController interface {
	// run starts the lease controller. It blocks until context is cancelled, or if the lease cannot be established.
	run(ctx context.Context)
}

// newV1BetaV1LeaseController creates a new lease controller.
//
// pingStatusFunc is a function that may block, but will be called every lease update interval, and will block updating,
// or establishing the lease. If the pingResult has no error and the function returns without error, the lease
// controller continues as normal. Otherwise, if it returns an error the lease controller will exit, and transition to
// the disabled state.
//
// getNodeFunc is a function that should return the last node observed in API server. This could be the return from
// a Create, or Update function, or Get the node itself. It should be non-blocking, as if it blocks, it will block
// creation of the lease controller. It expects errors to be one of context, or an error from
// k8s.io/apimachinery/pkg/api/errors.
//
// The lease controller will only exit if context is cancelled.
func newV1BetaV1LeaseController(
	client v1beta1.LeaseInterface,
	lease *coord.Lease,
	leaseRenewalInterval time.Duration,
	pingStatusFunc func(context.Context) (*pingResult, error),
	getNodeFunc func(context.Context) (*corev1.Node, error)) leaseController {

	if leaseRenewalInterval == 0 {
		panic("Lease renewal interval is 0")
	}

	lc := &v1Betav1LeaseController{
		client:               client,
		leaseRenewalInterval: leaseRenewalInterval,
		pingStatusFunc:       pingStatusFunc,
		getNodeFunc:          getNodeFunc,
	}
	if lease == nil {
		lc.lease = &coord.Lease{}
	} else {
		lc.lease = lease.DeepCopy()
	}

	return lc
}

type v1Betav1LeaseController struct {
	client               v1beta1.LeaseInterface
	leaseRenewalInterval time.Duration
	pingStatusFunc       func(context.Context) (*pingResult, error)
	getNodeFunc          func(context.Context) (*corev1.Node, error)
	lease                *coord.Lease
}

func (lc *v1Betav1LeaseController) run(ctx context.Context) {
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("leaseRenewalInterval", lc.leaseRenewalInterval))
	for {
		sleepTime := lc.leaseRenewalInterval

		err := lc.poll(ctx)
		if err != nil {
			if pkgerrors.Is(err, context.Canceled) {
				return
			}

			if seconds, delay := errors.SuggestsClientDelay(err); delay {
				sleepTime = time.Second * time.Duration(seconds)
			}
			log.G(ctx).WithError(err).WithField("sleepTime", sleepTime).Warn("Failed to update lease. Retrying")
		}
		timer := time.NewTimer(sleepTime)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

func (lc *v1Betav1LeaseController) poll(ctx context.Context) (retErr error) {
	ctx, span := trace.StartSpan(ctx, "v1Betav1LeaseController.poll")
	defer span.End()

	defer func() {
		span.SetStatus(retErr)
	}()

	pr, err := lc.pingStatusFunc(ctx)
	if err != nil {
		return fmt.Errorf("Received error when attempting to ascertain node status: %w", err)
	}

	if pr.error != nil {
		return newNodeNodeReady(pr)
	}

	lc.lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now()}
	// This is 25 due to historical reasons. It was supposed to be * 5, but...reasons
	d := int32(lc.leaseRenewalInterval.Seconds()) * 25
	lc.lease.Spec.LeaseDurationSeconds = &d

	serverNode, err := lc.getNodeFunc(ctx)
	if err != nil {
		return err
	}
	if serverNode == nil {
		return pkgerrors.New("servernode is null")
	}

	if lc.lease.Name == "" {
		lc.lease.Name = serverNode.Name
	}

	if lc.lease.Spec.HolderIdentity == nil {
		name := serverNode.Name
		lc.lease.Spec.HolderIdentity = &name
	}

	setOwnerReference(ctx, lc.lease, serverNode)
	ctx = span.WithFields(ctx, log.Fields{
		"lease.name": lc.lease.Name,
		"lease.time": lc.lease.Spec.RenewTime,
	})

	// This means the lease hasn't been created before in the API Server.
	if lc.lease.UID == "" {
	retry:
		l, err := lc.client.Create(ctx, lc.lease, metav1.CreateOptions{})
		if err != nil {
			log.G(ctx).WithError(err).Error("Failed to create new lease")
			// The node might have been running before. Try to recreate the lease.
			if errors.IsAlreadyExists(err) || errors.IsConflict(err) {
				log.G(ctx).WithError(err).Warn("Error creating lease, deleting and recreating")
				err = lc.client.Delete(ctx, lc.lease.Name, metav1.DeleteOptions{})
				if err != nil && !errors.IsNotFound(err) {
					log.G(ctx).WithError(err).Error("could not delete old node lease")
					return err
				}
				log.G(ctx).Info("Existing lease deleted, sleeping and retrying to create lease")
				sleep := time.NewTimer(100 * time.Millisecond)
				defer sleep.Stop()
				select {
				case <-sleep.C:
				case <-ctx.Done():
					return ctx.Err()
				}
				goto retry
			}
			return err
		}

		log.G(ctx).WithField("lease", l).Info("Created new lease")
		lc.lease = l
		return nil
	}

	// This has the error behaviour that if we run into a conflict, we will not delete the lease server side,
	// this behaviour will be fixed in the V1 Lease controller.
	newLease, err := lc.client.Update(ctx, lc.lease, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	lc.lease = newLease
	return nil
}

func setOwnerReference(ctx context.Context, lease *coord.Lease, serverNode *corev1.Node) {
	// Copied and pasted from: https://github.com/kubernetes/kubernetes/blob/442a69c3bdf6fe8e525b05887e57d89db1e2f3a5/pkg/kubelet/nodelease/controller.go#L213-L216
	// Setting owner reference needs node's UID. Note that it is different from
	// kubelet.nodeRef.UID. When lease is initially created, it is possible that
	// the connection between master and node is not ready yet. So try to set
	// owner reference every time when renewing the lease, until successful.
	//
	// We have a special case to deal with in the node may be deleted and
	// come back with a different UID. In this case the lease object should
	// be deleted due to a owner reference cascading deletion, and when we renew
	// lease again updateNodeLease will call ensureLease, and establish a new
	// lease with the right node ID
	if l := len(lease.OwnerReferences); l == 0 {
		lease.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: corev1.SchemeGroupVersion.WithKind("Node").Version,
				Kind:       corev1.SchemeGroupVersion.WithKind("Node").Kind,
				Name:       serverNode.Name,
				UID:        serverNode.UID,
			},
		}
	} else if l > 0 {
		var foundAnyNode bool
		for _, ref := range lease.OwnerReferences {
			if ref.APIVersion == corev1.SchemeGroupVersion.WithKind("Node").Version && ref.Kind == corev1.SchemeGroupVersion.WithKind("Node").Kind {
				foundAnyNode = true
				if serverNode.UID == ref.UID && serverNode.Name == ref.Name {
					return
				}

				log.G(ctx).WithFields(map[string]interface{}{
					"node.UID":  serverNode.UID,
					"ref.UID":   ref.UID,
					"node.Name": serverNode.Name,
					"ref.Name":  ref.Name,
				}).Warn("Found that lease had node in owner references that is not this node")
			}
		}
		if !foundAnyNode {
			log.G(ctx).WithField("ownerReferences", lease.OwnerReferences).Warn("Found that lease had owner references, but no nodes in owner references")
		}
	}
}

// nodeNodeReadyError indicates that the node was not ready
type nodeNodeReadyError struct {
	pingResult *pingResult
}

func newNodeNodeReady(pingResult *pingResult) error {
	return &nodeNodeReadyError{
		pingResult: pingResult,
	}
}

func (e *nodeNodeReadyError) Unwrap() error {
	return e.pingResult.error
}

func (e *nodeNodeReadyError) Is(target error) bool {
	_, ok := target.(*nodeNodeReadyError)
	return ok
}

func (e *nodeNodeReadyError) As(target error) bool {
	val, ok := target.(*nodeNodeReadyError)
	if ok {
		*val = *e
	}
	return ok
}

func (e *nodeNodeReadyError) Error() string {
	return fmt.Sprintf("New node not ready error: %s", e.pingResult.error)
}
