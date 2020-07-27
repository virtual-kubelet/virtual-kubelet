package api

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"

	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
)

// Controller is used for managing the HTTP API to serve pod routes.
// E.g. API's for `kubectl exec` and `kubectl logs`.
type Controller struct {
	cfg ControllerConfig
	srv *http.Server

	ready chan struct{}
	done  chan struct{}

	mu  sync.Mutex
	err error
}

// ControllerConfig holds options for the API controller
type ControllerConfig struct {
	// The listener that will serve pod routes
	// If this is not provided, the controller will attempt to create a listener on the default kubelet port (10250).
	PodListener net.Listener

	// The TLSConfig will be used to serve TLS on the listener if provided.
	// TLSConfig is required if PodListener is not set.
	TLSConfig *tls.Config

	// Config for route handlers
	PodHandler PodHandlerConfig

	// Enable debug routes
	EnableDebug bool
}

// NewController creates a `Controller`, which manages an HTTP server instance
// This does not run or start thing. To do that call `Run`
func NewController(cfg ControllerConfig) (*Controller, error) {
	if cfg.PodListener == nil && cfg.TLSConfig == nil {
		return nil, errdefs.InvalidInput("must provide either a listener and/or a TLS config to configure a listener: refusing to listen on non-TLS port")
	}

	mux := &http.ServeMux{}
	AttachPodRoutes(cfg.PodHandler, mux, cfg.EnableDebug)

	return &Controller{
		cfg:   cfg,
		srv:   &http.Server{Handler: mux, TLSConfig: cfg.TLSConfig},
		ready: make(chan struct{}),
		done:  make(chan struct{}),
	}, nil
}

// Run runs the controller
// If the controller was created without a listener configured, this is where the listener will be started.
//
// This function does not return until the server is shutdown.
// To shut down the server, you must cancel the past in context.
//
// You can check on the status of the controller with `Ready()` and `Done()`
func (c *Controller) Run(ctx context.Context) (retErr error) {
	defer func() {
		c.mu.Lock()
		c.err = retErr
		c.mu.Unlock()
		close(c.done)
	}()

	if c.cfg.PodListener == nil {
		l, err := net.Listen("tcp", ":10250")
		if err != nil {
			return err
		}
		c.cfg.PodListener = l
	}

	if c.cfg.TLSConfig != nil {
		c.cfg.PodListener = tls.NewListener(c.cfg.PodListener, c.cfg.TLSConfig)
	}

	go c.srv.Serve(c.cfg.PodListener)

	close(c.ready)

	<-ctx.Done()

	c.srv.Close()

	return ctx.Err()
}

// Ready returns a channel that will closed once the server is ready to serve content
func (c *Controller) Ready() <-chan struct{} {
	return c.ready
}

// Done returns a channel that will be closed once the server is no longer serving content.
func (c *Controller) Done() <-chan struct{} {
	return c.done
}

// Err returns whatever error is returned by `Run`
func (c *Controller) Err() error {
	c.mu.Lock()
	err := c.err
	c.mu.Unlock()
	return err
}
