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
	"strings"
	"sync"
	"testing"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"

	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watch "k8s.io/apimachinery/pkg/watch"
	testclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/retry"
)

func TestNodeRun(t *testing.T) {
	t.Run("WithoutLease", func(t *testing.T) { testNodeRun(t, false) })
	t.Run("WithLease", func(t *testing.T) { testNodeRun(t, true) })
}

func testNodeRun(t *testing.T, enableLease bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := testclient.NewSimpleClientset()

	testP := &testNodeProvider{NodeProvider: &NaiveNodeProvider{}}

	nodes := c.CoreV1().Nodes()
	leases := c.CoordinationV1().Leases(corev1.NamespaceNodeLease)

	interval := 1 * time.Millisecond
	opts := []NodeControllerOpt{
		WithNodePingInterval(interval),
		WithNodeStatusUpdateInterval(interval),
	}
	if enableLease {
		opts = append(opts, WithNodeEnableLeaseV1WithRenewInterval(leases, 40, interval))
	}
	testNode := testNode(t)
	// We have to refer to testNodeCopy during the course of the test. testNode is modified by the node controller
	// so it will trigger the race detector.
	testNodeCopy := testNode.DeepCopy()
	node, err := NewNodeController(testP, testNode, nodes, opts...)
	assert.NilError(t, err)

	defer func() {
		cancel()
		<-node.Done()
		assert.NilError(t, node.Err())
	}()

	go node.Run(ctx) //nolint:errcheck

	nw := makeWatch(ctx, t, nodes, testNodeCopy.Name)
	defer nw.Stop()
	nr := nw.ResultChan()

	lw := makeWatch(ctx, t, leases, testNodeCopy.Name)
	defer lw.Stop()
	lr := lw.ResultChan()

	var (
		lBefore      *coordinationv1.Lease
		nodeUpdates  int
		leaseUpdates int

		iters         = 50
		expectAtLeast = iters / 5
	)

	timeout := time.After(30 * time.Second)
	for i := 0; i < iters; i++ {
		var l *coordinationv1.Lease

		select {
		case <-timeout:
			t.Fatal("timed out waiting for expected events")
		case <-time.After(time.Second):
			t.Errorf("timeout waiting for event")
			continue
		case <-node.Done():
			t.Fatal(node.Err()) // if this returns at all it is an error regardless if err is nil
		case <-nr:
			nodeUpdates++
			continue
		case le := <-lr:
			l = le.Object.(*coordinationv1.Lease)
			leaseUpdates++

			assert.Assert(t, cmp.Equal(l.Spec.HolderIdentity != nil, true))
			assert.NilError(t, err)
			assert.Check(t, cmp.Equal(*l.Spec.HolderIdentity, testNodeCopy.Name))
			if lBefore != nil {
				assert.Check(t, before(lBefore.Spec.RenewTime.Time, l.Spec.RenewTime.Time))
			}

			lBefore = l
		}
	}

	lw.Stop()
	nw.Stop()

	assert.Check(t, atLeast(nodeUpdates, expectAtLeast))
	if enableLease {
		assert.Check(t, atLeast(leaseUpdates, expectAtLeast))
	} else {
		assert.Check(t, cmp.Equal(leaseUpdates, 0))
	}

	// trigger an async node status update
	n, err := nodes.Get(ctx, testNode.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	newCondition := corev1.NodeCondition{
		Type:               corev1.NodeConditionType("UPDATED"),
		LastTransitionTime: metav1.Now().Rfc3339Copy(),
	}
	n.Status.Conditions = append(n.Status.Conditions, newCondition)

	nw = makeWatch(ctx, t, nodes, testNodeCopy.Name)
	defer nw.Stop()
	nr = nw.ResultChan()

	testP.triggerStatusUpdate(n)

	eCtx, eCancel := context.WithTimeout(ctx, 10*time.Second)
	defer eCancel()

	select {
	case <-node.Done():
		t.Fatal(node.Err()) // if this returns at all it is an error regardless if err is nil
	case err := <-waitForEvent(eCtx, nr, func(e watch.Event) bool {
		node := e.Object.(*corev1.Node)
		if len(node.Status.Conditions) == 0 {
			return false
		}

		// Check if this is a node update we are looking for
		// Since node updates happen periodically there could be some that occur
		// before the status update that we are looking for happens.
		c := node.Status.Conditions[len(n.Status.Conditions)-1]
		if !c.LastTransitionTime.Equal(&newCondition.LastTransitionTime) {
			return false
		}
		if c.Type != newCondition.Type {
			return false
		}
		return true
	}):
		assert.NilError(t, err, "error waiting for updated node condition")
	}
}

func TestNodeCustomUpdateStatusErrorHandler(t *testing.T) {
	c := testclient.NewSimpleClientset()
	testP := &testNodeProvider{NodeProvider: &NaiveNodeProvider{}}
	nodes := c.CoreV1().Nodes()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	node, err := NewNodeController(testP, testNode(t), nodes,
		WithNodeStatusUpdateErrorHandler(func(_ context.Context, err error) error {
			cancel()
			return nil
		}),
	)
	assert.NilError(t, err)

	go node.Run(ctx) //nolint:errcheck

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	// wait for the node to be ready
	select {
	case <-timer.C:
		t.Fatal("timeout waiting for node to be ready")
	case <-node.Done():
		t.Fatalf("node.Run returned earlier than expected: %v", node.Err())
	case <-node.Ready():
	}

	err = nodes.Delete(ctx, node.serverNode.Name, metav1.DeleteOptions{})
	assert.NilError(t, err)

	testP.triggerStatusUpdate(node.serverNode.DeepCopy())

	timer = time.NewTimer(10 * time.Second)
	defer timer.Stop()

	select {
	case <-node.Done():
		assert.NilError(t, node.Err())
	case <-timer.C:
		t.Fatal("timeout waiting for node shutdown")
	}
}

func TestUpdateNodeStatus(t *testing.T) {
	n := testNode(t)
	n.Status.Conditions = append(n.Status.Conditions, corev1.NodeCondition{
		LastHeartbeatTime: metav1.Now().Rfc3339Copy(),
	})
	n.Status.Phase = corev1.NodePending
	nodes := testclient.NewSimpleClientset().CoreV1().Nodes()

	ctx := context.Background()
	_, err := updateNodeStatus(ctx, nodes, n.DeepCopy())
	assert.Equal(t, errors.IsNotFound(err), true, err)

	_, err = nodes.Create(ctx, n, metav1.CreateOptions{})
	assert.NilError(t, err)

	updated, err := updateNodeStatus(ctx, nodes, n.DeepCopy())
	assert.NilError(t, err)

	assert.NilError(t, err)
	assert.Check(t, cmp.DeepEqual(n.Status, updated.Status))

	n.Status.Phase = corev1.NodeRunning
	updated, err = updateNodeStatus(ctx, nodes, n.DeepCopy())
	assert.NilError(t, err)
	assert.Check(t, cmp.DeepEqual(n.Status, updated.Status))

	err = nodes.Delete(ctx, n.Name, metav1.DeleteOptions{})
	assert.NilError(t, err)

	_, err = nodes.Get(ctx, n.Name, metav1.GetOptions{})
	assert.Equal(t, errors.IsNotFound(err), true, err)

	_, err = updateNodeStatus(ctx, nodes, updated.DeepCopy())
	assert.Equal(t, errors.IsNotFound(err), true, err)
}

// TestPingAfterStatusUpdate checks that Ping continues to be called with the specified interval
// after a node status update occurs, when leases are disabled.
//
// Timing ratios used in this test:
// ping interval (10 ms)
// maximum allowed interval = 2.5 * ping interval
// status update interval = 6 * ping interval
//
// The allowed maximum time is 2.5 times the ping interval because
// the status update resets the ping interval timer, meaning
// that there can be a full two interval durations between
// successive calls to Ping. The extra half is to allow
// for timing variations when using such short durations.
//
// Once the node controller is ready:
// send status update after 10 * ping interval
// end test after another 10 * ping interval
func TestPingAfterStatusUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := testclient.NewSimpleClientset()
	nodes := c.CoreV1().Nodes()

	testP := &testNodeProviderPing{}

	interval := 10 * time.Millisecond
	maxAllowedInterval := time.Duration(2.5 * float64(interval.Nanoseconds()))

	opts := []NodeControllerOpt{
		WithNodePingInterval(interval),
		WithNodeStatusUpdateInterval(interval * time.Duration(6)),
	}

	testNode := testNode(t)
	testNodeCopy := testNode.DeepCopy()

	node, err := NewNodeController(testP, testNode, nodes, opts...)
	assert.NilError(t, err)

	go node.Run(ctx) //nolint:errcheck
	defer func() {
		cancel()
		<-node.Done()
		assert.NilError(t, node.Err())
	}()

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	// wait for the node to be ready
	select {
	case <-timer.C:
		t.Fatal("timeout waiting for node to be ready")
	case <-node.Done():
		t.Fatalf("node.Run returned earlier than expected: %v", node.Err())
	case <-node.Ready():
	}
	timer.Stop()

	notifyTimer := time.After(interval * time.Duration(10))
	<-notifyTimer
	testP.triggerStatusUpdate(testNodeCopy)

	endTimer := time.After(interval * time.Duration(10))
	<-endTimer

	testP.maxPingIntervalLock.Lock()
	defer testP.maxPingIntervalLock.Unlock()
	assert.Assert(t, testP.maxPingInterval < maxAllowedInterval, "maximum time between node pings (%v) was greater than the maximum expected interval (%v)", testP.maxPingInterval, maxAllowedInterval)
}

// Are annotations that were created before the VK existed preserved?
func TestBeforeAnnotationsPreserved(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := testclient.NewSimpleClientset()

	testP := &testNodeProvider{NodeProvider: &NaiveNodeProvider{}}

	nodes := c.CoreV1().Nodes()

	interval := 10 * time.Millisecond
	opts := []NodeControllerOpt{
		WithNodePingInterval(interval),
	}

	testNode := testNode(t)
	testNodeCreateCopy := testNode.DeepCopy()
	testNodeCreateCopy.Annotations = map[string]string{
		"beforeAnnotation": "value",
	}
	_, err := nodes.Create(ctx, testNodeCreateCopy, metav1.CreateOptions{})
	assert.NilError(t, err)

	// We have to refer to testNodeCopy during the course of the test. testNode is modified by the node controller
	// so it will trigger the race detector.
	testNodeCopy := testNode.DeepCopy()
	node, err := NewNodeController(testP, testNode, nodes, opts...)
	assert.NilError(t, err)

	defer func() {
		cancel()
		<-node.Done()
		assert.NilError(t, node.Err())
	}()

	go node.Run(ctx) //nolint:errcheck

	nw := makeWatch(ctx, t, nodes, testNodeCopy.Name)
	defer nw.Stop()
	nr := nw.ResultChan()

	t.Log("Waiting for node to exist")
	assert.NilError(t, <-waitForEvent(ctx, nr, func(e watch.Event) bool {
		return e.Object != nil
	}))

	testP.notifyNodeStatus(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"testAnnotation": "value",
			},
		},
	})

	assert.NilError(t, <-waitForEvent(ctx, nr, func(e watch.Event) bool {
		if e.Object == nil {
			return false
		}
		_, ok := e.Object.(*corev1.Node).Annotations["testAnnotation"]

		return ok
	}))

	newNode, err := nodes.Get(ctx, testNodeCopy.Name, emptyGetOptions)
	assert.NilError(t, err)

	assert.Assert(t, cmp.Contains(newNode.Annotations, "testAnnotation"))
	assert.Assert(t, cmp.Contains(newNode.Annotations, "beforeAnnotation"))
}

// Are conditions set by systems outside of VK preserved?
func TestManualConditionsPreserved(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := testclient.NewSimpleClientset()

	testP := &testNodeProvider{NodeProvider: &NaiveNodeProvider{}}

	nodes := c.CoreV1().Nodes()

	interval := 10 * time.Millisecond
	opts := []NodeControllerOpt{
		WithNodePingInterval(interval),
	}

	testNode := testNode(t)
	// We have to refer to testNodeCopy during the course of the test. testNode is modified by the node controller
	// so it will trigger the race detector.
	testNodeCopy := testNode.DeepCopy()
	node, err := NewNodeController(testP, testNode, nodes, opts...)
	assert.NilError(t, err)

	defer func() {
		cancel()
		<-node.Done()
		assert.NilError(t, node.Err())
	}()

	go node.Run(ctx) //nolint:errcheck

	nw := makeWatch(ctx, t, nodes, testNodeCopy.Name)
	defer nw.Stop()
	nr := nw.ResultChan()

	t.Log("Waiting for node to exist")
	assert.NilError(t, <-waitForEvent(ctx, nr, func(e watch.Event) bool {
		if e.Object == nil {
			return false
		}
		receivedNode := e.Object.(*corev1.Node)
		return len(receivedNode.Status.Conditions) == 0
	}))

	newNode, err := nodes.Get(ctx, testNodeCopy.Name, emptyGetOptions)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Len(newNode.Status.Conditions, 0))

	baseCondition := corev1.NodeCondition{
		Type:    "BaseCondition",
		Status:  "Ok",
		Reason:  "NA",
		Message: "This is the base condition. It is set by VK, and should always be there.",
	}

	testP.notifyNodeStatus(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"testAnnotation": "value",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				baseCondition,
			},
		},
	})

	// Wait for this (node with condition) to show up
	assert.NilError(t, <-waitForEvent(ctx, nr, func(e watch.Event) bool {
		receivedNode := e.Object.(*corev1.Node)
		for _, condition := range receivedNode.Status.Conditions {
			if condition.Type == baseCondition.Type {
				return true
			}
		}
		return false
	}))

	newNode, err = nodes.Get(ctx, testNodeCopy.Name, emptyGetOptions)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Len(newNode.Status.Conditions, 1))
	assert.Assert(t, cmp.Contains(newNode.Annotations, "testAnnotation"))

	// Add a new event manually
	manuallyAddedCondition := corev1.NodeCondition{
		Type:    "ManuallyAddedCondition",
		Status:  "Ok",
		Reason:  "NA",
		Message: "This is a manually added condition. Outside of VK. It should not be removed.",
	}
	assert.NilError(t, retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newNode, err = nodes.Get(ctx, testNodeCopy.Name, emptyGetOptions)
		if err != nil {
			return err
		}
		newNode.Annotations["manuallyAddedAnnotation"] = "value"
		newNode.Status.Conditions = append(newNode.Status.Conditions, manuallyAddedCondition)
		_, err = nodes.UpdateStatus(ctx, newNode, metav1.UpdateOptions{})
		return err
	}))

	assert.NilError(t, <-waitForEvent(ctx, nr, func(e watch.Event) bool {
		receivedNode := e.Object.(*corev1.Node)
		for _, condition := range receivedNode.Status.Conditions {
			if condition.Type == manuallyAddedCondition.Type {
				return true
			}
		}
		assert.Assert(t, cmp.Contains(receivedNode.Annotations, "testAnnotation"))
		assert.Assert(t, cmp.Contains(newNode.Annotations, "manuallyAddedAnnotation"))

		return false
	}))

	// Let's have the VK have a new condition.
	newCondition := corev1.NodeCondition{
		Type:    "NewCondition",
		Status:  "Ok",
		Reason:  "NA",
		Message: "This is a newly added condition. It should only show up *with* / *after* ManuallyAddedCondition. It is set by the VK.",
	}

	// Everything but node status is ignored here
	testP.notifyNodeStatus(&corev1.Node{
		// Annotations is left empty
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				baseCondition,
				newCondition,
			},
		},
	})
	i := 0
	assert.NilError(t, <-waitForEvent(ctx, nr, func(e watch.Event) bool {
		receivedNode := e.Object.(*corev1.Node)
		for _, condition := range receivedNode.Status.Conditions {
			if condition.Type == newCondition.Type {
				// Wait for 2 updates / patches
				if i > 2 {
					return true
				}
				i++
			}
		}
		return false
	}))

	// Make sure that all three conditions are there.
	newNode, err = nodes.Get(ctx, testNodeCopy.Name, emptyGetOptions)
	assert.NilError(t, err)
	seenConditionTypes := make([]corev1.NodeConditionType, len(newNode.Status.Conditions))
	for idx := range newNode.Status.Conditions {
		seenConditionTypes[idx] = newNode.Status.Conditions[idx].Type
	}
	assert.Assert(t, cmp.Contains(seenConditionTypes, baseCondition.Type))
	assert.Assert(t, cmp.Contains(seenConditionTypes, newCondition.Type))
	assert.Assert(t, cmp.Contains(seenConditionTypes, manuallyAddedCondition.Type))
	assert.Assert(t, cmp.Equal(newNode.Annotations["testAnnotation"], ""))
	assert.Assert(t, cmp.Contains(newNode.Annotations, "manuallyAddedAnnotation"))

	t.Log(newNode.Status.Conditions)
}

func TestNodePingSingleInflight(t *testing.T) {
	testCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const pingTimeout = 100 * time.Millisecond
	c := testclient.NewSimpleClientset()
	testP := &testNodeProviderPing{}

	calls := newWaitableInt()
	finished := newWaitableInt()

	ctx, cancel := context.WithTimeout(testCtx, time.Second)
	defer cancel()

	// The ping callback function is meant to block during the entire lifetime of the node ping controller.
	// The point is to check whether or it allows callbacks to stack up.
	testP.customPingFunction = func(context.Context) error {
		calls.increment()
		// This timer has to be longer than that of the context of the controller because we want to make sure
		// that goroutines are not allowed to stack up. If this exits as soon as that timeout is up, finished
		// will be incremented and we might miss goroutines stacking up, so we wait a tiny bit longer than
		// the nodePingController control loop (we wait 2 seconds, the control loop only lasts 1 second)

		// This is the context tied to the lifetime of the node ping controller, not the context created
		// for the specific invocation of this ping function
		<-ctx.Done()
		finished.increment()
		return nil
	}

	nodes := c.CoreV1().Nodes()

	testNode := testNode(t)

	node, err := NewNodeController(testP, testNode, nodes, WithNodePingInterval(10*time.Millisecond), WithNodePingTimeout(pingTimeout))
	assert.NilError(t, err)

	start := time.Now()
	go node.nodePingController.Run(ctx)
	firstPing, err := node.nodePingController.getResult(ctx)
	assert.NilError(t, err)
	timeTakenToCompleteFirstPing := time.Since(start)
	assert.Assert(t, timeTakenToCompleteFirstPing < pingTimeout*5, "Time taken to complete first ping: %v", timeTakenToCompleteFirstPing)

	assert.Assert(t, cmp.Error(firstPing.error, context.DeadlineExceeded.Error()))
	assert.Assert(t, cmp.Equal(1, calls.read()))
	assert.Assert(t, cmp.Equal(0, finished.read()))

	// Wait until the first sleep finishes (the test context is done)
	assert.NilError(t, finished.until(testCtx, func(i int) bool { return i > 0 }))

	// Assert we didn't stack up goroutines, and that the one goroutine in flight finishd
	assert.Assert(t, cmp.Equal(1, calls.read()))
	assert.Assert(t, cmp.Equal(1, finished.read()))

}

func testNode(t *testing.T) *corev1.Node {
	n := &corev1.Node{}
	n.Name = strings.ToLower(t.Name())
	return n
}

type testNodeProvider struct {
	NodeProvider
	statusHandlers []func(*corev1.Node)
	// Callback to VK
	notifyNodeStatus func(*corev1.Node)
}

func (p *testNodeProvider) NotifyNodeStatus(ctx context.Context, h func(*corev1.Node)) {
	p.notifyNodeStatus = h
}

func (p *testNodeProvider) triggerStatusUpdate(n *corev1.Node) {
	for _, h := range p.statusHandlers {
		h(n)
	}
	p.notifyNodeStatus(n)
}

// testNodeProviderPing tracks the maximum time interval between calls to Ping
type testNodeProviderPing struct {
	testNodeProvider
	customPingFunction  func(context.Context) error
	lastPingTime        time.Time
	maxPingIntervalLock sync.Mutex
	maxPingInterval     time.Duration
}

func (tnp *testNodeProviderPing) Ping(ctx context.Context) error {
	if tnp.customPingFunction != nil {
		return tnp.customPingFunction(ctx)
	}

	now := time.Now()
	if tnp.lastPingTime.IsZero() {
		tnp.lastPingTime = now
		return nil
	}
	tnp.maxPingIntervalLock.Lock()
	defer tnp.maxPingIntervalLock.Unlock()
	if now.Sub(tnp.lastPingTime) > tnp.maxPingInterval {
		tnp.maxPingInterval = now.Sub(tnp.lastPingTime)
	}
	tnp.lastPingTime = now
	return nil
}

type watchGetter interface {
	Watch(context.Context, metav1.ListOptions) (watch.Interface, error)
}

func makeWatch(ctx context.Context, t *testing.T, wc watchGetter, name string) watch.Interface {
	t.Helper()

	w, err := wc.Watch(ctx, metav1.ListOptions{FieldSelector: "name=" + name})
	assert.NilError(t, err)
	return w
}

func atLeast(x, atLeast int) cmp.Comparison {
	return func() cmp.Result {
		if x < atLeast {
			return cmp.ResultFailureTemplate(failTemplate("<"), map[string]interface{}{"x": x, "y": atLeast})
		}
		return cmp.ResultSuccess
	}
}

func before(x, y time.Time) cmp.Comparison {
	return func() cmp.Result {
		if x.Before(y) {
			return cmp.ResultSuccess
		}
		return cmp.ResultFailureTemplate(failTemplate(">="), map[string]interface{}{"x": x, "y": y})
	}
}

// waitForEvent waits for the `check` function to return true
// `check` is run when an event is received
// Cancelling the context will cancel the wait, with the context error sent on
// the returned channel.
func waitForEvent(ctx context.Context, chEvent <-chan watch.Event, check func(watch.Event) bool) <-chan error {
	chErr := make(chan error, 1)
	go func() {

		for {
			select {
			case e := <-chEvent:
				if check(e) {
					chErr <- nil
					return
				}
			case <-ctx.Done():
				chErr <- ctx.Err()
				return
			}
		}
	}()

	return chErr
}

func failTemplate(op string) string {
	return `
			{{- .Data.x}} (
				{{- with callArg 0 }}{{ formatNode . }} {{end -}}
				{{- printf "%T" .Data.x -}}
			) ` + op + ` {{ .Data.y}} (
				{{- with callArg 1 }}{{ formatNode . }} {{end -}}
				{{- printf "%T" .Data.y -}}
			)`
}
