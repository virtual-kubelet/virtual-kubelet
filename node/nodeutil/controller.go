package nodeutil

import (
	"context"
	"fmt"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/node"
)

// ControllerManager helps manage the startup/shutdown procedure for other controllers.
// It is intended as a convenience to reduce boiler plate code for starting up controllers.
//
// Must be created with constructor `NewControllerManager`.
type ControllerManager struct {
	nc *node.NodeController
	pc *node.PodController

	ready chan struct{}
	done  chan struct{}
	err   error
}

// NewControllerManager creates a new ControllerManager.
func NewControllerManager(nc *node.NodeController, pc *node.PodController) *ControllerManager {
	return &ControllerManager{
		nc:    nc,
		pc:    pc,
		ready: make(chan struct{}),
		done:  make(chan struct{}),
	}
}

// NodeController returns the configured node controller.
func (c *ControllerManager) NodeController() *node.NodeController {
	return c.nc
}

// PodController returns the configured pod controller.
func (c *ControllerManager) PodController() *node.PodController {
	return c.pc
}

// Run starts all the underlying controllers
func (c *ControllerManager) Run(ctx context.Context, workers int) (retErr error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go c.pc.Run(ctx, workers) // nolint:errcheck

	defer func() {
		cancel()

		<-c.pc.Done()

		c.err = retErr
		close(c.done)
	}()

	select {
	case <-ctx.Done():
		return c.err
	case <-c.pc.Ready():
	case <-c.pc.Done():
		return c.pc.Err()
	}

	go c.nc.Run(ctx) // nolint:errcheck

	defer func() {
		cancel()
		<-c.nc.Done()
	}()

	select {
	case <-ctx.Done():
		c.err = ctx.Err()
		return c.err
	case <-c.nc.Ready():
	case <-c.nc.Done():
		return c.nc.Err()
	}

	close(c.ready)

	select {
	case <-c.nc.Done():
		cancel()
		return c.nc.Err()
	case <-c.pc.Done():
		cancel()
		return c.pc.Err()
	}
}

// WaitReady waits for the specified timeout for the controller to be ready.
//
// The timeout is for convenience so the caller doesn't have to juggle an extra context.
func (c *ControllerManager) WaitReady(ctx context.Context, timeout time.Duration) error {
	if timeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	select {
	case <-c.ready:
		return nil
	case <-c.done:
		return fmt.Errorf("controller exited before ready: %w", c.err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Ready returns a channel that will be closed after the controller is ready.
func (c *ControllerManager) Ready() <-chan struct{} {
	return c.ready
}

// Done returns a channel that will be closed when the controller has exited.
func (c *ControllerManager) Done() <-chan struct{} {
	return c.done
}

// Err returns any error that occurred with the controller.
//
// This always return nil before `<-Done()`.
func (c *ControllerManager) Err() error {
	select {
	case <-c.Done():
		return c.err
	default:
		return nil
	}
}
