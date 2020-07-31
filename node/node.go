// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package node

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"golang.org/x/sync/singleflight"
	coord "k8s.io/api/coordination/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/typed/coordination/v1beta1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
)

const (
	// Annotation with the JSON-serialized last applied node conditions. Based on kube ctl apply. Used to calculate
	// the three-way patch
	virtualKubeletLastNodeAppliedNodeStatus = "virtual-kubelet.io/last-applied-node-status"
	virtualKubeletLastNodeAppliedObjectMeta = "virtual-kubelet.io/last-applied-object-meta"
)

var (
	ErrLeaseControllerAlreadyConfigured = pkgerrors.New("Lease controller already configured")
)

// NodeProvider is the interface used for registering a node and updating its
// status in Kubernetes.
//
// Note: Implementers can choose to manage a node themselves, in which case
// it is not needed to provide an implementation for this interface.
type NodeProvider interface { //nolint:golint
	// Ping checks if the node is still active.
	// This is intended to be lightweight as it will be called periodically as a
	// heartbeat to keep the node marked as ready in Kubernetes.
	Ping(context.Context) error

	// NotifyNodeStatus is used to asynchronously monitor the node.
	// The passed in callback should be called any time there is a change to the
	// node's status.
	// This will generally trigger a call to the Kubernetes API server to update
	// the status.
	//
	// NotifyNodeStatus should not block callers.
	NotifyNodeStatus(ctx context.Context, cb func(*corev1.Node))
}

// NewNodeController creates a new node leaseController.
// This does not have any side-effects on the system or kubernetes.
//
// Use the node's `Run` method to register and run the loops to update the node
// in Kubernetes.
//
// Note: When if there are multiple NodeControllerOpts which apply against the same
// underlying options, the last NodeControllerOpt will win.
func NewNodeController(p NodeProvider, node *corev1.Node, nodes v1.NodeInterface, opts ...NodeControllerOpt) (*NodeController, error) {
	n := &NodeController{
		p:                    p,
		n:                    node,
		nodes:                nodes,
		chReady:              make(chan struct{}),
		firstPingCompleted:   make(chan struct{}),
		lastPingErrorUpdated: make(chan struct{}),
	}
	for _, o := range opts {
		if err := o(n); err != nil {
			return nil, pkgerrors.Wrap(err, "error applying node option")
		}
	}
	return n, nil
}

// NodeControllerOpt are the functional options used for configuring a node
type NodeControllerOpt func(*NodeController) error // nolint: golint

// WithNodeEnableLeaseV1Beta1 enables support for v1beta1 leases.
// If client is nil, leases will not be enabled.
// If baseLease is nil, a default base lease will be used.
//
// The lease will be updated after each successful node ping. To change the
// lease update interval, you must set the node ping interval.
// See WithNodePingInterval().
//
// This also affects the frequency of node status updates:
//   - When leases are *not* enabled (or are disabled due to no support on the cluster)
//     the node status is updated at every ping interval.
//   - When node leases are enabled, node status updates are controlled by the
//     node status update interval option.
// To set a custom node status update interval, see WithNodeStatusUpdateInterval().
func WithNodeEnableLeaseV1Beta1(client v1beta1.LeaseInterface, baseLease *coord.Lease) NodeControllerOpt {
	return WithCustomNodeEnableLeaseV1Beta1(client, baseLease, 0, 0)
}

// WithNodeEnableLeaseV1 enables support for coordinaion v1 leases.
// If client is nil, a panic will occur
// If baseLease is nil, a default base lease will be used.
//
// Specifying 0 will set the defaults
//
// This also affects the frequency of node status updates:
//   - When leases are *not* enabled (or are disabled due to no support on the cluster)
//     the node status is updated at every ping interval.
//   - When node leases are enabled, node status updates are controlled by the
//     node status update interval option.
// To set a custom node status update interval, see WithNodeStatusUpdateInterval().
//
func WithCustomNodeEnableLeaseV1Beta1(client v1beta1.LeaseInterface, baseLease *coord.Lease, leaseDurationSeconds int32, renewIntervalFraction float32) NodeControllerOpt {
	return func(n *NodeController) error {
		if n.leaseController != nil {
			return ErrLeaseControllerAlreadyConfigured
		}
		n.leaseController = &leaseController{
			leaseClient:          client,
			leaseDurationSeconds: leaseDurationSeconds,
			renewInterval:        time.Second * time.Duration(renewIntervalFraction*float32(leaseDurationSeconds)),
			baseLease:            baseLease,
			firstSyncComplete:    make(chan struct{}),
		}
		return nil
	}
}

// WithNodePingTimeout limits the amount of time that the virtual kubelet will wait for the node provider to
// respond to the ping callback. If it does not return within this time, it will be considered an error
// condition
func WithNodePingTimeout(timeout time.Duration) NodeControllerOpt {
	return func(n *NodeController) error {
		n.pingTimeout = &timeout
		return nil
	}
}

// WithNodePingInterval sets the interval between checking for node statuses via Ping()
// If node leases are not supported (or not enabled), this is the frequency
// with which the node status will be updated in Kubernetes.
func WithNodePingInterval(d time.Duration) NodeControllerOpt {
	return func(n *NodeController) error {
		n.pingInterval = d
		return nil
	}
}

// WithNodeStatusUpdateInterval sets the interval for updating node status
// This is only used when leases are supported and only for updating the actual
// node status, not the node lease.
// When node leases are not enabled (or are not supported on the cluster) this
// has no affect and node status is updated on the "ping" interval.
func WithNodeStatusUpdateInterval(d time.Duration) NodeControllerOpt {
	return func(n *NodeController) error {
		n.statusInterval = d
		return nil
	}
}

// WithNodeStatusUpdateErrorHandler adds an error handler for cases where there is an error
// when updating the node status.
// This allows the caller to have some control on how errors are dealt with when
// updating a node's status.
//
// The error passed to the handler will be the error received from kubernetes
// when updating node status.
func WithNodeStatusUpdateErrorHandler(h ErrorHandler) NodeControllerOpt {
	return func(n *NodeController) error {
		n.nodeStatusUpdateErrorHandler = h
		return nil
	}
}

// ErrorHandler is a type of function used to allow callbacks for handling errors.
// It is expected that if a nil error is returned that the error is handled and
// progress can continue (or a retry is possible).
type ErrorHandler func(context.Context, error) error

// NodeController deals with creating and managing a node object in Kubernetes.
// It can register a node with Kubernetes and periodically update its status.
// NodeController manages a single node entity.
type NodeController struct { // nolint: golint
	p NodeProvider
	n *corev1.Node

	leaseController *leaseController
	nodes           v1.NodeInterface

	pingInterval   time.Duration
	statusInterval time.Duration
	chStatusUpdate chan *corev1.Node

	nodeStatusUpdateErrorHandler ErrorHandler

	chReady chan struct{}

	// What was the last error we got from the node upon ping
	lastPingErrorLock    sync.Mutex
	lastPingError        error
	lastPingErrorUpdated chan struct{}
	firstPingCompleted   chan struct{}
	pingTimeout          *time.Duration
}

// The default intervals used for lease and status updates.
const (
	DefaultPingInterval         = 10 * time.Second
	DefaultStatusUpdateInterval = 1 * time.Minute
)

// Run registers the node in kubernetes and starts loops for updating the node
// status in Kubernetes.
//
// The node status must be updated periodically in Kubernetes to keep the node
// active. Newer versions of Kubernetes support node leases, which are
// essentially light weight pings. Older versions of Kubernetes require updating
// the node status periodically.
//
// If Kubernetes supports node leases this will use leases with a much slower
// node status update (because some things still expect the node to be updated
// periodically), otherwise it will only use node status update with the configured
// ping interval.
func (n *NodeController) Run(ctx context.Context) error {
	if n.pingInterval == time.Duration(0) {
		n.pingInterval = DefaultPingInterval
	}
	if n.statusInterval == time.Duration(0) {
		n.statusInterval = DefaultStatusUpdateInterval
	}

	n.chStatusUpdate = make(chan *corev1.Node, 1)
	n.p.NotifyNodeStatus(ctx, func(node *corev1.Node) {
		n.chStatusUpdate <- node
	})

	if err := n.ensureNode(ctx); err != nil {
		return err
	}

	if n.leaseController != nil {
		if n.leaseController.leaseDurationSeconds == 0 {
			n.leaseController.leaseDurationSeconds = int32(n.pingInterval) * 25
		}
		if n.leaseController.renewInterval == 0 {
			n.leaseController.renewInterval = n.pingInterval
		}

		n.leaseController.holderIdentity = n.n.Name
		n.leaseController.nodesClient = n.nodes
		n.leaseController.shouldUpdateLease = func(ctx context.Context) bool {
			_, lastPingStatus := n.getLastPingStatus(ctx)
			return lastPingStatus == nil
		}
	}

	log.G(ctx).Debug("Created node lease")
	return n.controlLoop(ctx)
}

func (n *NodeController) ensureNode(ctx context.Context) (err error) {
	ctx, span := trace.StartSpan(ctx, "node.ensureNode")
	defer span.End()
	defer func() {
		span.SetStatus(err)
	}()

	err = n.updateStatus(ctx, true)
	if err == nil || !errors.IsNotFound(err) {
		return err
	}

	node, err := n.nodes.Create(ctx, n.n, metav1.CreateOptions{})
	if err != nil {
		return pkgerrors.Wrap(err, "error registering node with kubernetes")
	}
	n.n = node

	return nil
}

// Ready returns a channel that gets closed when the node is fully up and
// running. Note that if there is an error on startup this channel will never
// be started.
func (n *NodeController) Ready() <-chan struct{} {
	return n.chReady
}

// pingLoop does one thing. It pings the provider. It does this until context is cancelled.
func (n *NodeController) pingLoop(ctx context.Context) {
	const key = "key"
	sf := &singleflight.Group{}

	// 1. If the node is "stuck" and not responding to pings, we want to set the status
	//    to that the node provider has timed out responding to pings
	// 2. We want it so that the context is cancelled, and whatever the node might have
	//    been stuck on uses context so it might be unstuck
	// 3. We want to retry pinging the node, but we do not ever want more than one
	//    ping in flight at a time.

	mkContextFunc := context.WithCancel

	if n.pingTimeout != nil {
		mkContextFunc = func(ctx2 context.Context) (context.Context, context.CancelFunc) {
			return context.WithTimeout(ctx2, *n.pingTimeout)
		}
	}

	checkFunc := func(ctx context.Context) {
		ctx, cancel := mkContextFunc(ctx)
		defer cancel()
		ctx, span := trace.StartSpan(ctx, "node.pingLoop")
		defer span.End()
		doChan := sf.DoChan(key, func() (interface{}, error) {
			ctx, span := trace.StartSpan(ctx, "node.pingNode")
			defer span.End()
			err := n.p.Ping(ctx)
			span.SetStatus(err)
			return nil, err
		})

		var err error
		select {
		case <-ctx.Done():
			err = ctx.Err()
			log.G(ctx).WithError(err).Warn("Failed to ping node due to context cancellation")
		case result := <-doChan:
			err = result.Err
		}

		n.lastPingErrorLock.Lock()
		n.lastPingError = err
		close(n.lastPingErrorUpdated)
		n.lastPingErrorUpdated = make(chan struct{})
		n.lastPingErrorLock.Unlock()
		span.SetStatus(err)
	}

	// Run the first check manually
	checkFunc(ctx)

	close(n.firstPingCompleted)

	wait.UntilWithContext(ctx, checkFunc, n.pingInterval)
}

// Returns the result of the last node ping, and a channel, which will be closed when the
// status is once again updated.
func (n *NodeController) getLastPingStatus(ctx context.Context) (chan struct{}, error) {
	select {
	case <-n.firstPingCompleted:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	n.lastPingErrorLock.Lock()
	defer n.lastPingErrorLock.Unlock()
	return n.lastPingErrorUpdated, n.lastPingError
}

func (n *NodeController) getNodeStatusUpdateTimer(ctx context.Context) *time.Timer {
	if n.leaseController.getLeasesWorking(ctx) {
		log.G(ctx).WithField("statusInterval", n.statusInterval).Debug("leases are working, setting up timer")
		return time.NewTimer(n.statusInterval)
	}

	if n.leaseController == nil {
		log.G(ctx).WithField("pingInterval", n.pingInterval).Debug("leases not enabled, setting up timer")
		return time.NewTimer(n.pingInterval)
	}

	log.G(ctx).WithField("pingInterval", n.pingInterval).Debug("leases are not working, setting up timer")
	return time.NewTimer(n.pingInterval)
}

func (n *NodeController) nodeStatusUpdateLoop(ctx context.Context) {
	// There are three cases:
	// 1. The provider sent us an update. We must immediately process that
	// 2. The node provider ping is failing. We must wait for that to stop failing
	//    before proceeding
	// 3. Leases are disabled / not working, and we should update the status on api server
	//    in pingInterval duration
	// 4. Leases are working fine, and we should update the status at the node status interval
	loop := func(ctx context.Context) {
		ctx, span := trace.StartSpan(ctx, "node.nodeStatusUpdateLoop.loop")
		defer span.End()
		nextPingStatus, lastPingStatus := n.getLastPingStatus(ctx)

		var c <-chan time.Time
		if lastPingStatus == nil {
			// if last ping status passed, we do not need to perform the nextPingStatus fallthrough check
			nextPingStatus = nil
			timer := n.getNodeStatusUpdateTimer(ctx)
			defer timer.Stop()
			c = timer.C
		} else {
			log.G(ctx).WithError(lastPingStatus).Debug("Last ping status failed, not scheduling node status update")
		}
		select {
		case <-ctx.Done():
			return
		case <-c:
			err := n.updateStatus(ctx, false)
			if err != nil {
				log.G(ctx).WithError(err).Error("Error handling node status update")
				span.SetStatus(err)
			} else {
				log.G(ctx).Debug("Performed scheduled node update")
			}
		case <-nextPingStatus:
			// The node failed to respond to our last ping, so let's wait for it to be updated
			log.G(ctx).Debug("Received next ping status")
			return
		case updated := <-n.chStatusUpdate:
			log.G(ctx).Debug("Received node status update")
			n.n.Status = updated.Status
			n.n.ObjectMeta = metav1.ObjectMeta{
				Annotations: updated.Annotations,
				Labels:      updated.Labels,
				Name:        n.n.ObjectMeta.Name,
				Namespace:   n.n.ObjectMeta.Namespace,
				UID:         n.n.ObjectMeta.UID,
			}
			err := n.updateStatus(ctx, false)
			if err != nil {
				log.G(ctx).WithError(err).Error("Error handling node status update")
				span.SetStatus(err)
			}
		}
	}

	wait.UntilWithContext(ctx, loop, time.Duration(0))
}

func (n *NodeController) controlLoop(ctx context.Context) error {
	close(n.chReady)

	group := &wait.Group{}
	group.StartWithContext(ctx, n.pingLoop)
	group.StartWithContext(ctx, n.nodeStatusUpdateLoop)
	if n.leaseController != nil {
		group.StartWithContext(ctx, n.leaseController.run)
	}
	group.Wait()
	return nil
}

func (n *NodeController) updateStatus(ctx context.Context, skipErrorCb bool) (err error) {
	ctx, span := trace.StartSpan(ctx, "node.updateStatus")
	defer span.End()
	defer func() {
		span.SetStatus(err)
	}()

	updateNodeStatusHeartbeat(n.n)

	node, err := updateNodeStatus(ctx, n.nodes, n.n)
	if err != nil {
		if skipErrorCb || n.nodeStatusUpdateErrorHandler == nil {
			return err
		}
		if err := n.nodeStatusUpdateErrorHandler(ctx, err); err != nil {
			return err
		}

		node, err = updateNodeStatus(ctx, n.nodes, n.n)
		if err != nil {
			return err
		}
	}

	n.n = node
	return nil
}

// just so we don't have to allocate this on every get request
var emptyGetOptions = metav1.GetOptions{}

func prepareThreewayPatchBytesForNodeStatus(nodeFromProvider, apiServerNode *corev1.Node) ([]byte, error) {
	// We use these two values to calculate a patch. We use a three-way patch in order to avoid
	// causing state regression server side. For example, let's consider the scenario:
	/*
		UML Source:
		@startuml
		participant VK
		participant K8s
		participant ExternalUpdater
		note right of VK: Updates internal node conditions to [A, B]
		VK->K8s: Patch Upsert [A, B]
		note left of K8s: Node conditions are [A, B]
		ExternalUpdater->K8s: Patch Upsert [C]
		note left of K8s: Node Conditions are [A, B, C]
		note right of VK: Updates internal node conditions to [A]
		VK->K8s: Patch: delete B, upsert A\nThis is where things go wrong,\nbecause the patch is written to replace all node conditions\nit overwrites (drops) [C]
		note left of K8s: Node Conditions are [A]\nNode condition C from ExternalUpdater is no longer present
		@enduml
			     ,--.                                                        ,---.          ,---------------.
		     |VK|                                                        |K8s|          |ExternalUpdater|
		     `+-'                                                        `-+-'          `-------+-------'
		      |  ,------------------------------------------!.             |                    |
		      |  |Updates internal node conditions to [A, B]|_\            |                    |
		      |  `--------------------------------------------'            |                    |
		      |                     Patch Upsert [A, B]                    |                    |
		      | ----------------------------------------------------------->                    |
		      |                                                            |                    |
		      |                              ,--------------------------!. |                    |
		      |                              |Node conditions are [A, B]|_\|                    |
		      |                              `----------------------------'|                    |
		      |                                                            |  Patch Upsert [C]  |
		      |                                                            | <-------------------
		      |                                                            |                    |
		      |                           ,-----------------------------!. |                    |
		      |                           |Node Conditions are [A, B, C]|_\|                    |
		      |                           `-------------------------------'|                    |
		      |  ,---------------------------------------!.                |                    |
		      |  |Updates internal node conditions to [A]|_\               |                    |
		      |  `-----------------------------------------'               |                    |
		      | Patch: delete B, upsert A                                  |                    |
		      | This is where things go wrong,                             |                    |
		      | because the patch is written to replace all node conditions|                    |
		      | it overwrites (drops) [C]                                  |                    |
		      | ----------------------------------------------------------->                    |
		      |                                                            |                    |
		     ,----------------------------------------------------------!. |                    |
		     |Node Conditions are [A]                                   |_\|                    |
		     |Node condition C from ExternalUpdater is no longer present  ||                    |
		     `------------------------------------------------------------'+-.          ,-------+-------.
		     |VK|                                                        |K8s|          |ExternalUpdater|
		     `--'                                                        `---'          `---------------'
	*/
	// In order to calculate that last patch to delete B, and upsert C, we need to know that C was added by
	// "someone else". So, we keep track of our last applied value, and our current value. We then generate
	// our patch based on the diff of these and *not* server side state.
	oldVKStatus, ok1 := apiServerNode.Annotations[virtualKubeletLastNodeAppliedNodeStatus]
	oldVKObjectMeta, ok2 := apiServerNode.Annotations[virtualKubeletLastNodeAppliedObjectMeta]

	oldNode := corev1.Node{}
	// Check if there were no labels, which means someone else probably created the node, or this is an upgrade. Either way, we will consider
	// ourselves as never having written the node object before, so oldNode will be left empty. We will overwrite values if
	// our new node conditions / status / objectmeta have them
	if ok1 && ok2 {
		err := json.Unmarshal([]byte(oldVKObjectMeta), &oldNode.ObjectMeta)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "Cannot unmarshal old node object metadata (key: %q): %q", virtualKubeletLastNodeAppliedObjectMeta, oldVKObjectMeta)
		}
		err = json.Unmarshal([]byte(oldVKStatus), &oldNode.Status)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "Cannot unmarshal old node status (key: %q): %q", virtualKubeletLastNodeAppliedNodeStatus, oldVKStatus)
		}
	}

	// newNode is the representation of the node the provider "wants"
	newNode := corev1.Node{}
	newNode.ObjectMeta = simplestObjectMetadata(&apiServerNode.ObjectMeta, &nodeFromProvider.ObjectMeta)
	nodeFromProvider.Status.DeepCopyInto(&newNode.Status)

	// virtualKubeletLastNodeAppliedObjectMeta must always be set before virtualKubeletLastNodeAppliedNodeStatus,
	// otherwise we capture virtualKubeletLastNodeAppliedNodeStatus in virtualKubeletLastNodeAppliedObjectMeta,
	// which is wrong
	virtualKubeletLastNodeAppliedObjectMetaBytes, err := json.Marshal(newNode.ObjectMeta)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Cannot marshal object meta from provider")
	}
	newNode.Annotations[virtualKubeletLastNodeAppliedObjectMeta] = string(virtualKubeletLastNodeAppliedObjectMetaBytes)

	virtualKubeletLastNodeAppliedNodeStatusBytes, err := json.Marshal(newNode.Status)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Cannot marshal node status from provider")
	}
	newNode.Annotations[virtualKubeletLastNodeAppliedNodeStatus] = string(virtualKubeletLastNodeAppliedNodeStatusBytes)
	// Generate three way patch from oldNode -> newNode, without deleting the changes in api server
	// Should we use the Kubernetes serialization / deserialization libraries here?
	oldNodeBytes, err := json.Marshal(oldNode)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Cannot marshal old node bytes")
	}
	newNodeBytes, err := json.Marshal(newNode)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Cannot marshal new node bytes")
	}
	apiServerNodeBytes, err := json.Marshal(apiServerNode)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Cannot marshal api server node")
	}
	schema, err := strategicpatch.NewPatchMetaFromStruct(&corev1.Node{})
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Cannot get patch schema from node")
	}
	return strategicpatch.CreateThreeWayMergePatch(oldNodeBytes, newNodeBytes, apiServerNodeBytes, schema, true)
}

// updateNodeStatus triggers an update to the node status in Kubernetes.
// It first fetches the current node details and then sets the status according
// to the passed in node object.
//
// If you use this function, it is up to you to synchronize this with other operations.
// This reduces the time to second-level precision.
func updateNodeStatus(ctx context.Context, nodes v1.NodeInterface, nodeFromProvider *corev1.Node) (_ *corev1.Node, retErr error) {
	ctx, span := trace.StartSpan(ctx, "UpdateNodeStatus")
	defer func() {
		span.End()
		span.SetStatus(retErr)
	}()

	var updatedNode *corev1.Node
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		apiServerNode, err := nodes.Get(ctx, nodeFromProvider.Name, emptyGetOptions)
		if err != nil {
			return err
		}
		ctx = addNodeAttributes(ctx, span, apiServerNode)
		log.G(ctx).Debug("got node from api server")

		patchBytes, err := prepareThreewayPatchBytesForNodeStatus(nodeFromProvider, apiServerNode)
		if err != nil {
			return pkgerrors.Wrap(err, "Cannot generate patch")
		}
		log.G(ctx).WithError(err).WithField("patch", string(patchBytes)).Debug("Generated three way patch")

		updatedNode, err = nodes.Patch(ctx, nodeFromProvider.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")
		if err != nil {
			// We cannot wrap this error because the kubernetes error module doesn't understand wrapping
			log.G(ctx).WithField("patch", string(patchBytes)).WithError(err).Warn("Failed to patch node status")
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	log.G(ctx).WithField("node.resourceVersion", updatedNode.ResourceVersion).
		WithField("node.Status.Conditions", updatedNode.Status.Conditions).
		Debug("updated node status in api server")
	return updatedNode, nil
}

func updateNodeStatusHeartbeat(n *corev1.Node) {
	now := metav1.NewTime(time.Now())
	for i := range n.Status.Conditions {
		n.Status.Conditions[i].LastHeartbeatTime = now
	}
}

// NaiveNodeProvider is a basic node provider that only uses the passed in context
// on `Ping` to determine if the node is healthy.
type NaiveNodeProvider struct{}

// Ping just implements the NodeProvider interface.
// It returns the error from the passed in context only.
func (NaiveNodeProvider) Ping(ctx context.Context) error {
	return ctx.Err()
}

// NotifyNodeStatus implements the NodeProvider interface.
//
// This NaiveNodeProvider does not support updating node status and so this
// function is a no-op.
func (NaiveNodeProvider) NotifyNodeStatus(ctx context.Context, f func(*corev1.Node)) {
}

type taintsStringer []corev1.Taint

func (t taintsStringer) String() string {
	var s string
	for _, taint := range t {
		if s == "" {
			s = taint.Key + "=" + taint.Value + ":" + string(taint.Effect)
		} else {
			s += ", " + taint.Key + "=" + taint.Value + ":" + string(taint.Effect)
		}
	}
	return s
}

func addNodeAttributes(ctx context.Context, span trace.Span, n *corev1.Node) context.Context {
	return span.WithFields(ctx, log.Fields{
		"node.UID":     string(n.UID),
		"node.name":    n.Name,
		"node.cluster": n.ClusterName,
		"node.taints":  taintsStringer(n.Spec.Taints),
	})
}

// Provides the simplest object metadata to match the previous object. Name, namespace, UID. It copies labels and
// annotations from the second object if defined. It exempts the patch metadata
func simplestObjectMetadata(baseObjectMeta, objectMetaWithLabelsAndAnnotations *metav1.ObjectMeta) metav1.ObjectMeta {
	ret := metav1.ObjectMeta{
		Namespace:   baseObjectMeta.Namespace,
		Name:        baseObjectMeta.Name,
		UID:         baseObjectMeta.UID,
		Annotations: make(map[string]string),
	}
	if objectMetaWithLabelsAndAnnotations != nil {
		if objectMetaWithLabelsAndAnnotations.Labels != nil {
			ret.Labels = objectMetaWithLabelsAndAnnotations.Labels
		} else {
			ret.Labels = make(map[string]string)
		}
		if objectMetaWithLabelsAndAnnotations.Annotations != nil {
			// We want to copy over all annotations except the special embedded ones.
			for key := range objectMetaWithLabelsAndAnnotations.Annotations {
				if key == virtualKubeletLastNodeAppliedNodeStatus || key == virtualKubeletLastNodeAppliedObjectMeta {
					continue
				}
				ret.Annotations[key] = objectMetaWithLabelsAndAnnotations.Annotations[key]
			}
		}
	}
	return ret
}
