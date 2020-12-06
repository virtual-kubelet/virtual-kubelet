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
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
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

// NodeProvider is the interface used for registering a node and updating its
// status in Kubernetes.
//
// Note: Implementers can choose to manage a node themselves, in which case
// it is not needed to provide an implementation for this interface.
type NodeProvider interface { // nolint:golint
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

// NewNodeController creates a new node controller.
// This does not have any side-effects on the system or kubernetes.
//
// Use the node's `Run` method to register and run the loops to update the node
// in Kubernetes.
//
// Note: When if there are multiple NodeControllerOpts which apply against the same
// underlying options, the last NodeControllerOpt will win.
func NewNodeController(p NodeProvider, node *corev1.Node, nodes v1.NodeInterface, opts ...NodeControllerOpt) (*NodeController, error) {
	n := &NodeController{
		p:          p,
		serverNode: node,
		nodes:      nodes,
		chReady:    make(chan struct{}),
	}
	for _, o := range opts {
		if err := o(n); err != nil {
			return nil, pkgerrors.Wrap(err, "error applying node option")
		}
	}

	if n.pingInterval == time.Duration(0) {
		n.pingInterval = DefaultPingInterval
	}
	if n.statusInterval == time.Duration(0) {
		n.statusInterval = DefaultStatusUpdateInterval
	}

	n.nodePingController = newNodePingController(n.p, n.pingInterval, n.pingTimeout)

	return n, nil
}

// NodeControllerOpt are the functional options used for configuring a node
type NodeControllerOpt func(*NodeController) error // nolint:golint

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
	return func(n *NodeController) error {
		n.leases = client
		n.lease = baseLease
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
type NodeController struct { // nolint:golint
	p NodeProvider

	// serverNode should only be written to on initialization, or as the result of node creation.
	serverNode *corev1.Node

	leases v1beta1.LeaseInterface
	nodes  v1.NodeInterface

	disableLease   bool
	pingInterval   time.Duration
	statusInterval time.Duration
	lease          *coord.Lease
	chStatusUpdate chan *corev1.Node

	nodeStatusUpdateErrorHandler ErrorHandler

	chReady chan struct{}

	nodePingController *nodePingController
	pingTimeout        *time.Duration
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
	n.chStatusUpdate = make(chan *corev1.Node, 1)
	n.p.NotifyNodeStatus(ctx, func(node *corev1.Node) {
		n.chStatusUpdate <- node
	})

	providerNode := n.serverNode.DeepCopy()

	if err := n.ensureNode(ctx, providerNode); err != nil {
		return err
	}

	if n.leases == nil {
		n.disableLease = true
		return n.controlLoop(ctx, providerNode)
	}

	n.lease = newLease(ctx, n.lease, n.serverNode, n.pingInterval)

	l, err := ensureLease(ctx, n.leases, n.lease)
	if err != nil {
		if !errors.IsNotFound(err) {
			return pkgerrors.Wrap(err, "error creating node lease")
		}
		log.G(ctx).Info("Node leases not supported, falling back to only node status updates")
		n.disableLease = true
	}
	n.lease = l

	log.G(ctx).Debug("Created node lease")
	return n.controlLoop(ctx, providerNode)
}

func (n *NodeController) ensureNode(ctx context.Context, providerNode *corev1.Node) (err error) {
	ctx, span := trace.StartSpan(ctx, "node.ensureNode")
	defer span.End()
	defer func() {
		span.SetStatus(err)
	}()

	err = n.updateStatus(ctx, providerNode, true)
	if err == nil || !errors.IsNotFound(err) {
		return err
	}

	node, err := n.nodes.Create(ctx, n.serverNode, metav1.CreateOptions{})
	if err != nil {
		return pkgerrors.Wrap(err, "error registering node with kubernetes")
	}

	n.serverNode = node
	// Bad things will happen if the node is deleted in k8s and recreated by someone else
	// we rely on this persisting
	providerNode.ObjectMeta.Name = node.Name
	providerNode.ObjectMeta.Namespace = node.Namespace
	providerNode.ObjectMeta.UID = node.UID

	return nil
}

// Ready returns a channel that gets closed when the node is fully up and
// running. Note that if there is an error on startup this channel will never
// be started.
func (n *NodeController) Ready() <-chan struct{} {
	return n.chReady
}

func (n *NodeController) controlLoop(ctx context.Context, providerNode *corev1.Node) error {
	pingTimer := time.NewTimer(n.pingInterval)
	defer pingTimer.Stop()

	statusTimer := time.NewTimer(n.statusInterval)
	defer statusTimer.Stop()
	timerResetDuration := n.statusInterval
	if n.disableLease {
		// when resetting the timer after processing a status update, reset it to the ping interval
		// (since it will be the ping timer as serverNode.disableLease == true)
		timerResetDuration = n.pingInterval

		// hack to make sure this channel always blocks since we won't be using it
		if !statusTimer.Stop() {
			<-statusTimer.C
		}
	}

	close(n.chReady)

	group := &wait.Group{}
	group.StartWithContext(ctx, n.nodePingController.run)
	defer group.Wait()

	loop := func() bool {
		ctx, span := trace.StartSpan(ctx, "node.controlLoop.loop")
		defer span.End()

		select {
		case <-ctx.Done():
			return true
		case updated := <-n.chStatusUpdate:
			var t *time.Timer
			if n.disableLease {
				t = pingTimer
			} else {
				t = statusTimer
			}

			log.G(ctx).Debug("Received node status update")
			// Performing a status update so stop/reset the status update timer in this
			// branch otherwise there could be an unnecessary status update.
			if !t.Stop() {
				<-t.C
			}

			providerNode.Status = updated.Status
			providerNode.ObjectMeta.Annotations = updated.Annotations
			providerNode.ObjectMeta.Labels = updated.Labels
			if err := n.updateStatus(ctx, providerNode, false); err != nil {
				log.G(ctx).WithError(err).Error("Error handling node status update")
			}
			t.Reset(timerResetDuration)
		case <-statusTimer.C:
			if err := n.updateStatus(ctx, providerNode, false); err != nil {
				log.G(ctx).WithError(err).Error("Error handling node status update")
			}
			statusTimer.Reset(n.statusInterval)
		case <-pingTimer.C:
			if err := n.handlePing(ctx, providerNode); err != nil {
				log.G(ctx).WithError(err).Error("Error while handling node ping")
			} else {
				log.G(ctx).Debug("Successful node ping")
			}
			pingTimer.Reset(n.pingInterval)
		}
		return false
	}

	for {
		shouldTerminate := loop()
		if shouldTerminate {
			return nil
		}
	}
}

func (n *NodeController) handlePing(ctx context.Context, providerNode *corev1.Node) (retErr error) {
	ctx, span := trace.StartSpan(ctx, "node.handlePing")
	defer span.End()
	defer func() {
		span.SetStatus(retErr)
	}()

	result, err := n.nodePingController.getResult(ctx)
	if err != nil {
		err = pkgerrors.Wrap(err, "error while fetching result of node ping")
		return err
	}

	if result.error != nil {
		err = pkgerrors.Wrap(err, "node ping returned error on ping")
		return err
	}

	if n.disableLease {
		return n.updateStatus(ctx, providerNode, false)
	}

	// TODO(Sargun): Pass down the result / timestamp so we can accurately track when the ping actually occurred
	return n.updateLease(ctx)
}

func (n *NodeController) updateLease(ctx context.Context) error {
	l, err := updateNodeLease(ctx, n.leases, newLease(ctx, n.lease, n.serverNode, n.pingInterval))
	if err != nil {
		return err
	}

	n.lease = l
	return nil
}

func (n *NodeController) updateStatus(ctx context.Context, providerNode *corev1.Node, skipErrorCb bool) (err error) {
	ctx, span := trace.StartSpan(ctx, "node.updateStatus")
	defer span.End()
	defer func() {
		span.SetStatus(err)
	}()

	updateNodeStatusHeartbeat(providerNode)

	node, err := updateNodeStatus(ctx, n.nodes, providerNode)
	if err != nil {
		if skipErrorCb || n.nodeStatusUpdateErrorHandler == nil {
			return err
		}
		if err := n.nodeStatusUpdateErrorHandler(ctx, err); err != nil {
			return err
		}

		// This might have recreated the node, which may cause problems with our leases until a node update succeeds
		node, err = updateNodeStatus(ctx, n.nodes, providerNode)
		if err != nil {
			return err
		}
	}

	n.serverNode = node
	return nil
}

func ensureLease(ctx context.Context, leases v1beta1.LeaseInterface, lease *coord.Lease) (*coord.Lease, error) {
	l, err := leases.Create(ctx, lease, metav1.CreateOptions{})
	if err != nil {
		switch {
		case errors.IsNotFound(err):
			log.G(ctx).WithError(err).Info("Node lease not supported")
			return nil, err
		case errors.IsAlreadyExists(err), errors.IsConflict(err):
			log.G(ctx).WithError(err).Warn("Error creating lease, deleting and recreating")
			if err := leases.Delete(ctx, lease.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				log.G(ctx).WithError(err).Error("could not delete old node lease")
				return nil, pkgerrors.Wrap(err, "old lease exists but could not delete it")
			}
			l, err = leases.Create(ctx, lease, metav1.CreateOptions{})
		}
	}

	return l, err
}

// updateNodeLease updates the node lease.
//
// If this function returns an errors.IsNotFound(err) error, this likely means
// that node leases are not supported, if this is the case, call updateNodeStatus
// instead.
func updateNodeLease(ctx context.Context, leases v1beta1.LeaseInterface, lease *coord.Lease) (*coord.Lease, error) {
	ctx, span := trace.StartSpan(ctx, "node.UpdateNodeLease")
	defer span.End()

	ctx = span.WithFields(ctx, log.Fields{
		"lease.name": lease.Name,
		"lease.time": lease.Spec.RenewTime,
	})

	if lease.Spec.LeaseDurationSeconds != nil {
		ctx = span.WithField(ctx, "lease.expiresSeconds", *lease.Spec.LeaseDurationSeconds)
	}

	l, err := leases.Update(ctx, lease, metav1.UpdateOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			log.G(ctx).Debug("lease not found")
			l, err = ensureLease(ctx, leases, lease)
		}
		if err != nil {
			span.SetStatus(err)
			return nil, err
		}
		log.G(ctx).Debug("created new lease")
	} else {
		log.G(ctx).Debug("updated lease")
	}

	return l, nil
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

// This will return a new lease. It will either update base lease (and the set the renewal time appropriately), or create a brand new lease
func newLease(ctx context.Context, base *coord.Lease, serverNode *corev1.Node, leaseRenewalInterval time.Duration) *coord.Lease {
	var lease *coord.Lease
	if base == nil {
		lease = &coord.Lease{}
	} else {
		lease = base.DeepCopy()
	}

	lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now()}

	if lease.Spec.LeaseDurationSeconds == nil {
		// This is 25 due to historical reasons. It was supposed to be * 5, but...reasons
		d := int32(leaseRenewalInterval.Seconds()) * 25
		lease.Spec.LeaseDurationSeconds = &d
	}

	if lease.Name == "" {
		lease.Name = serverNode.Name
	}
	if lease.Spec.HolderIdentity == nil {
		// Let's do a copy here
		name := serverNode.Name
		lease.Spec.HolderIdentity = &name
	}

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
					return lease
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
			log.G(ctx).Warn("Found that lease had owner references, but no nodes in owner references")
		}
	}

	return lease
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
func (n NaiveNodeProvider) NotifyNodeStatus(_ context.Context, _ func(*corev1.Node)) {
}

// NaiveNodeProviderV2 is like NaiveNodeProvider except it supports accepting node status updates.
// It must be used with as a pointer and must be created with `NewNaiveNodeProvider`
type NaiveNodeProviderV2 struct {
	notify      func(*corev1.Node)
	updateReady chan struct{}
}

// Ping just implements the NodeProvider interface.
// It returns the error from the passed in context only.
func (*NaiveNodeProviderV2) Ping(ctx context.Context) error {
	return ctx.Err()
}

// NotifyNodeStatus implements the NodeProvider interface.
//
// NaiveNodeProvider does not support updating node status unless created with `NewNaiveNodeProvider`
// Otherwise this is a no-op
func (n *NaiveNodeProviderV2) NotifyNodeStatus(_ context.Context, f func(*corev1.Node)) {
	n.notify = f
	// This is a little sloppy and assumes `NotifyNodeStatus` is only called once, which is indeed currently true.
	// The reason a channel is preferred here is so we can use a context in `UpdateStatus` to cancel waiting for this.
	close(n.updateReady)
}

// UpdateStatus sends a node status update to the node controller
func (n *NaiveNodeProviderV2) UpdateStatus(ctx context.Context, node *corev1.Node) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-n.updateReady:
	}

	n.notify(node)
	return nil
}

// NewNaiveNodeProvider creates a new NaiveNodeProviderV2
// You must use this to create a NaiveNodeProviderV2 if you want to be able to send node status updates to the node
// controller.
func NewNaiveNodeProvider() *NaiveNodeProviderV2 {
	return &NaiveNodeProviderV2{
		updateReady: make(chan struct{}),
	}
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
