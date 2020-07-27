package node

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
)

// ControllerManager can be used to setup pod and node controllers without requiring a bunch of boiler plate code.
//
// Create one with `NewControllerManager`, then start it with `ControllerManager.Start()`
type ControllerManager struct {
	pc *PodController
	nc *NodeController
	ac *api.Controller

	pcWorkers      int
	startupTimeout time.Duration

	mu  sync.Mutex
	err error

	done  chan struct{}
	ready chan struct{}
}

// Run starts up the controllers and waits for them to finish.
//
// This only returns once all controllers have finished.
// You can check on the status asynchronously with `Ready()`, `Done()`, and `Err()`.
func (m *ControllerManager) Run(ctx context.Context) (retErr error) {
	ctx, cancel := context.WithCancel(ctx)

	defer func() {
		cancel()

		m.mu.Lock()
		m.err = retErr
		m.mu.Unlock()

		close(m.done)
		log.G(ctx).WithError(retErr).Debug("ControllerManager exiting")
	}()

	go func() {
		err := m.pc.Run(ctx, m.pcWorkers)
		log.G(ctx).WithError(err).Debug("Pod controller exited")
		cancel()
	}()

	if err := waitReady(ctx, m.pc, m.startupTimeout); err != nil {
		return errors.Wrap(err, "error waiting for pod controller to be ready")
	}
	log.G(ctx).Debug("Pod controller ready")

	go func() {
		err := m.nc.Run(ctx)
		log.G(ctx).WithError(err).Debug("Node controller exited")
		cancel()
	}()

	if err := waitReady(ctx, m.nc, m.startupTimeout); err != nil {
		return errors.Wrap(err, "error waiting for node controller to be ready")
	}
	log.G(ctx).Debug("Node controller ready")

	go func() {
		err := m.ac.Run(ctx)
		log.G(ctx).WithError(err).Debug("API controller exited")
		cancel()
	}()

	if err := waitReady(ctx, m.ac, m.startupTimeout); err != nil {
		return errors.Wrap(err, "error waiting for API controller")
	}
	log.G(ctx).Debug("API controller ready")

	close(m.ready)
	log.G(ctx).Debug("Controller manger ready")

	select {
	case <-m.nc.Done():
		return m.nc.Err()
	case <-m.pc.Done():
		return m.pc.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Ready returns a channel that gets closed when the node is fully up and
// running. Note that if there is an error on startup this channel will never
// be started.
func (n *ControllerManager) Ready() <-chan struct{} {
	return n.ready
}

// Done returns a channel that gets closed when the manager is no longer operating.
// This means both the pod controller and the node controller are shutdown.
func (n *ControllerManager) Done() <-chan struct{} {
	return n.done
}

// Err returns whatever error is returned by `Run()`.
// It allows the caller to check on the status of the without having to deal with setting up their own goroutines.
// If this returns a non-nil error, the  manager is no longer operating.
func (n *ControllerManager) Err() error {
	n.mu.Lock()
	err := n.err
	n.mu.Unlock()
	return err
}

func waitReady(ctx context.Context, w waiter, dur time.Duration) error {
	if dur > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, dur)
		defer cancel()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-w.Ready():
		return w.Err()
	case <-w.Done():
		return w.Err()
	}
}

type waiter interface {
	Ready() <-chan struct{}
	Done() <-chan struct{}
	Err() error
}

// NewControllerManager creates a new ControllerManager.
// The manager is not active until you call `Run`
func NewControllerManager(cfg ControllerManagerConfig) (*ControllerManager, error) {
	return &ControllerManager{
		nc:             cfg.NodeController,
		pc:             cfg.PodController,
		ac:             cfg.APIController,
		startupTimeout: cfg.StartupTimeout,
		pcWorkers:      cfg.PodControllerWorkers,
		done:           make(chan struct{}),
		ready:          make(chan struct{}),
	}, nil
}

// ControllerManagerConfig is how configuration is passed when creating a new manager.
type ControllerManagerConfig struct {
	NodeController *NodeController
	PodController  *PodController
	APIController  *api.Controller

	// Number of workers for syncing between the pod provider and the Kubernetes API server.
	PodControllerWorkers int

	// Time to wait for each controller to be ready
	StartupTimeout time.Duration
}
