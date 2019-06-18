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
	"testing"
	"time"

	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
	coord "k8s.io/api/coordination/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watch "k8s.io/apimachinery/pkg/watch"
	testclient "k8s.io/client-go/kubernetes/fake"
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
	leases := c.Coordination().Leases(corev1.NamespaceNodeLease)

	interval := 1 * time.Millisecond
	opts := []NodeControllerOpt{
		WithNodePingInterval(interval),
		WithNodeStatusUpdateInterval(interval),
	}
	if enableLease {
		opts = append(opts, WithNodeEnableLeaseV1Beta1(leases, nil))
	}
	node, err := NewNodeController(testP, testNode(t), nodes, opts...)
	assert.NilError(t, err)

	chErr := make(chan error)
	defer func() {
		cancel()
		assert.NilError(t, <-chErr)
	}()

	go func() {
		chErr <- node.Run(ctx)
		close(chErr)
	}()

	nw := makeWatch(t, nodes, node.n.Name)
	defer nw.Stop()
	nr := nw.ResultChan()

	lw := makeWatch(t, leases, node.n.Name)
	defer lw.Stop()
	lr := lw.ResultChan()

	var (
		lBefore      *coord.Lease
		nodeUpdates  int
		leaseUpdates int

		iters         = 50
		expectAtLeast = iters / 5
	)

	timeout := time.After(30 * time.Second)
	for i := 0; i < iters; i++ {
		var l *coord.Lease

		select {
		case <-timeout:
			t.Fatal("timed out waiting for expected events")
		case <-time.After(time.Second):
			t.Errorf("timeout waiting for event")
			continue
		case err := <-chErr:
			t.Fatal(err) // if this returns at all it is an error regardless if err is nil
		case <-nr:
			nodeUpdates++
			continue
		case le := <-lr:
			l = le.Object.(*coord.Lease)
			leaseUpdates++

			assert.Assert(t, cmp.Equal(l.Spec.HolderIdentity != nil, true))
			assert.Check(t, cmp.Equal(*l.Spec.HolderIdentity, node.n.Name))
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
	n := node.n.DeepCopy()
	newCondition := corev1.NodeCondition{
		Type:               corev1.NodeConditionType("UPDATED"),
		LastTransitionTime: metav1.Now().Rfc3339Copy(),
	}
	n.Status.Conditions = append(n.Status.Conditions, newCondition)

	nw = makeWatch(t, nodes, node.n.Name)
	defer nw.Stop()
	nr = nw.ResultChan()

	testP.triggerStatusUpdate(n)

	eCtx, eCancel := context.WithTimeout(ctx, 10*time.Second)
	defer eCancel()

	select {
	case err := <-chErr:
		t.Fatal(err) // if this returns at all it is an error regardless if err is nil
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

	chErr := make(chan error, 1)
	go func() {
		chErr <- node.Run(ctx)
	}()

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	// wait for the node to be ready
	select {
	case <-timer.C:
		t.Fatal("timeout waiting for node to be ready")
	case <-chErr:
		t.Fatalf("node.Run returned earlier than expected: %v", err)
	case <-node.Ready():
	}

	err = nodes.Delete(node.n.Name, nil)
	assert.NilError(t, err)

	testP.triggerStatusUpdate(node.n.DeepCopy())

	timer = time.NewTimer(10 * time.Second)
	defer timer.Stop()

	select {
	case err := <-chErr:
		assert.Equal(t, err, nil)
	case <-timer.C:
		t.Fatal("timeout waiting for node shutdown")
	}
}

func TestEnsureLease(t *testing.T) {
	c := testclient.NewSimpleClientset().Coordination().Leases(corev1.NamespaceNodeLease)
	n := testNode(t)
	ctx := context.Background()

	lease := newLease(nil)
	setLeaseAttrs(lease, n, 1*time.Second)

	l1, err := ensureLease(ctx, c, lease.DeepCopy())
	assert.NilError(t, err)
	assert.Check(t, timeEqual(l1.Spec.RenewTime.Time, lease.Spec.RenewTime.Time))

	l1.Spec.RenewTime.Time = time.Now().Add(1 * time.Second)
	l2, err := ensureLease(ctx, c, l1.DeepCopy())
	assert.NilError(t, err)
	assert.Check(t, timeEqual(l2.Spec.RenewTime.Time, l1.Spec.RenewTime.Time))
}

func TestUpdateNodeStatus(t *testing.T) {
	n := testNode(t)
	n.Status.Conditions = append(n.Status.Conditions, corev1.NodeCondition{
		LastHeartbeatTime: metav1.Now().Rfc3339Copy(),
	})
	n.Status.Phase = corev1.NodePending
	nodes := testclient.NewSimpleClientset().CoreV1().Nodes()

	ctx := context.Background()
	updated, err := UpdateNodeStatus(ctx, nodes, n.DeepCopy())
	assert.Equal(t, errors.IsNotFound(err), true, err)

	_, err = nodes.Create(n)
	assert.NilError(t, err)

	updated, err = UpdateNodeStatus(ctx, nodes, n.DeepCopy())
	assert.NilError(t, err)

	assert.NilError(t, err)
	assert.Check(t, cmp.DeepEqual(n.Status, updated.Status))

	n.Status.Phase = corev1.NodeRunning
	updated, err = UpdateNodeStatus(ctx, nodes, n.DeepCopy())
	assert.NilError(t, err)
	assert.Check(t, cmp.DeepEqual(n.Status, updated.Status))

	err = nodes.Delete(n.Name, nil)
	assert.NilError(t, err)

	_, err = nodes.Get(n.Name, metav1.GetOptions{})
	assert.Equal(t, errors.IsNotFound(err), true, err)

	_, err = UpdateNodeStatus(ctx, nodes, updated.DeepCopy())
	assert.Equal(t, errors.IsNotFound(err), true, err)
}

func TestUpdateNodeLease(t *testing.T) {
	leases := testclient.NewSimpleClientset().Coordination().Leases(corev1.NamespaceNodeLease)
	lease := newLease(nil)
	n := testNode(t)
	setLeaseAttrs(lease, n, 0)

	ctx := context.Background()
	l, err := UpdateNodeLease(ctx, leases, lease)
	assert.NilError(t, err)
	assert.Equal(t, l.Name, lease.Name)
	assert.Assert(t, cmp.DeepEqual(l.Spec.HolderIdentity, lease.Spec.HolderIdentity))

	compare, err := leases.Get(l.Name, emptyGetOptions)
	assert.NilError(t, err)
	assert.Equal(t, l.Spec.RenewTime.Time.Unix(), compare.Spec.RenewTime.Time.Unix())
	assert.Equal(t, compare.Name, lease.Name)
	assert.Assert(t, cmp.DeepEqual(compare.Spec.HolderIdentity, lease.Spec.HolderIdentity))

	l.Spec.RenewTime.Time = time.Now().Add(10 * time.Second)

	compare, err = UpdateNodeLease(ctx, leases, l.DeepCopy())
	assert.NilError(t, err)
	assert.Equal(t, compare.Spec.RenewTime.Time.Unix(), l.Spec.RenewTime.Time.Unix())
	assert.Equal(t, compare.Name, lease.Name)
	assert.Assert(t, cmp.DeepEqual(compare.Spec.HolderIdentity, lease.Spec.HolderIdentity))
}

func testNode(t *testing.T) *corev1.Node {
	n := &corev1.Node{}
	n.Name = strings.ToLower(t.Name())
	return n
}

type testNodeProvider struct {
	NodeProvider
	statusHandlers []func(*corev1.Node)
}

func (p *testNodeProvider) NotifyNodeStatus(ctx context.Context, h func(*corev1.Node)) {
	p.statusHandlers = append(p.statusHandlers, h)
}

func (p *testNodeProvider) triggerStatusUpdate(n *corev1.Node) {
	for _, h := range p.statusHandlers {
		h(n)
	}
}

type watchGetter interface {
	Watch(metav1.ListOptions) (watch.Interface, error)
}

func makeWatch(t *testing.T, wc watchGetter, name string) watch.Interface {
	t.Helper()

	w, err := wc.Watch(metav1.ListOptions{FieldSelector: "name=" + name})
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

func timeEqual(x, y time.Time) cmp.Comparison {
	return func() cmp.Result {
		if x.Equal(y) {
			return cmp.ResultSuccess
		}
		return cmp.ResultFailureTemplate(failTemplate("!="), map[string]interface{}{"x": x, "y": y})
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
