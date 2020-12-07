package node

import (
	"context"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/internal/lock"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"golang.org/x/sync/singleflight"
	"k8s.io/apimachinery/pkg/util/wait"
)

// nodePingController is responsible for node pinging behaviour
type nodePingController struct {
	nodeProvider NodeProvider
	pingInterval time.Duration
	pingTimeout  *time.Duration
	cond         lock.MonitorVariable
}

// pingResult encapsulates the result of the last ping. It is the time the ping was started, and the error.
// If there is a timeout, the error will be context.DeadlineExceeded
type pingResult struct {
	time  time.Time
	error error
}

// newNodePingController creates a new node ping controller. pingInterval must be non-zero. Optionally, a timeout may be specfied on
// how long to wait for the provider to respond
func newNodePingController(node NodeProvider, pingInterval time.Duration, timeout *time.Duration) *nodePingController {
	if pingInterval == 0 {
		panic("Node ping interval is 0")
	}

	if timeout != nil && *timeout == 0 {
		panic("Node ping timeout is 0")
	}

	return &nodePingController{
		nodeProvider: node,
		pingInterval: pingInterval,
		pingTimeout:  timeout,
		cond:         lock.NewMonitorVariable(),
	}
}

// Run runs the controller until context is cancelled
func (npc *nodePingController) Run(ctx context.Context) {
	const key = "key"
	sf := &singleflight.Group{}

	// 1. If the node is "stuck" and not responding to pings, we want to set the status
	//    to that the node provider has timed out responding to pings
	// 2. We want it so that the context is cancelled, and whatever the node might have
	//    been stuck on uses context so it might be unstuck
	// 3. We want to retry pinging the node, but we do not ever want more than one
	//    ping in flight at a time.

	mkContextFunc := context.WithCancel

	if npc.pingTimeout != nil {
		mkContextFunc = func(ctx2 context.Context) (context.Context, context.CancelFunc) {
			return context.WithTimeout(ctx2, *npc.pingTimeout)
		}
	}

	checkFunc := func(ctx context.Context) {
		ctx, cancel := mkContextFunc(ctx)
		defer cancel()
		ctx, span := trace.StartSpan(ctx, "node.pingLoop")
		defer span.End()
		doChan := sf.DoChan(key, func() (interface{}, error) {
			now := time.Now()
			ctx, span := trace.StartSpan(ctx, "node.pingNode")
			defer span.End()
			err := npc.nodeProvider.Ping(ctx)
			span.SetStatus(err)
			return now, err
		})

		var pingResult pingResult
		select {
		case <-ctx.Done():
			pingResult.error = ctx.Err()
			log.G(ctx).WithError(pingResult.error).Warn("Failed to ping node due to context cancellation")
		case result := <-doChan:
			pingResult.error = result.Err
			pingResult.time = result.Val.(time.Time)
		}

		npc.cond.Set(&pingResult)
		span.SetStatus(pingResult.error)
	}

	// Run the first check manually
	checkFunc(ctx)

	wait.UntilWithContext(ctx, checkFunc, npc.pingInterval)
}

// GetResult returns the current ping result in a non-blocking fashion except for the first ping. It waits for the
// first ping to be successful before returning. If the context is cancelled while waiting for that value, it will
// return immediately.
func (npc *nodePingController) getResult(ctx context.Context) (*pingResult, error) {
	sub := npc.cond.Subscribe()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-sub.NewValueReady():
	}

	return sub.Value().Value.(*pingResult), nil
}
