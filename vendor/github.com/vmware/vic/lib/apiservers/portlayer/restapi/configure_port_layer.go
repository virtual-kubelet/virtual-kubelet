// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package restapi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/swag"
	"github.com/tylerb/graceful"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/handlers"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/options"
	"github.com/vmware/vic/lib/portlayer"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/pkg/version"
	"github.com/vmware/vic/pkg/vsphere/session"
)

// This file is safe to edit. Once it exists it will not be overwritten

type handler interface {
	Configure(api *operations.PortLayerAPI, handlerCtx *handlers.HandlerContext)
}

var portlayerhandlers = []handler{
	&handlers.StorageHandlersImpl{},
	&handlers.MiscHandlersImpl{},
	&handlers.ScopesHandlersImpl{},
	&handlers.ContainersHandlersImpl{},
	&handlers.InteractionHandlersImpl{},
	&handlers.LoggingHandlersImpl{},
	&handlers.KvHandlersImpl{},
	&handlers.EventsHandlerImpl{},
	&handlers.TaskHandlersImpl{},
}

var apiServers []*graceful.Server

const stopTimeout = time.Second * 3

func configureFlags(api *operations.PortLayerAPI) {
	api.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{
		{
			LongDescription:  "Port Layer Options",
			Options:          options.PortLayerOptions,
			ShortDescription: "Port Layer Options",
		},
	}
}

func configureAPI(api *operations.PortLayerAPI) http.Handler {
	api.Logger = log.Printf

	ctx := context.Background()

	sessionconfig := &session.Config{
		Service:        options.PortLayerOptions.SDK,
		Insecure:       options.PortLayerOptions.Insecure,
		Keepalive:      options.PortLayerOptions.Keepalive,
		DatacenterPath: options.PortLayerOptions.DatacenterPath,
		ClusterPath:    options.PortLayerOptions.ClusterPath,
		PoolPath:       options.PortLayerOptions.PoolPath,
		DatastorePath:  options.PortLayerOptions.DatastorePath,
		UserAgent:      version.UserAgent("vic-engine"),
	}

	sess, err := session.NewSession(sessionconfig).Create(ctx)
	if err != nil {
		log.Fatalf("configure_port_layer ERROR: %s", err)
	}

	// Configure the func invoked if the PL panics or is restarted by vic-init
	api.ServerShutdown = func() {
		log.Infof("Shutting down port-layer-server")

		// stop the event collectors
		collectors := exec.Config.EventManager.Collectors()
		for _, c := range collectors {
			c.Stop()
		}

		// Logout the session
		if err := sess.Logout(ctx); err != nil {
			log.Warnf("unable to log out of session: %s", err)
		}
	}

	// initialize the port layer
	if err = portlayer.Init(ctx, sess); err != nil {
		log.Fatalf("could not initialize port layer: %s", err)
	}

	// configure the api here
	api.ServeError = errors.ServeError

	// FIXME: after updated go-openapi/runtime vendor code, revert ByteStreamConsumer() back to runtime.ByteStreamConsumer()
	api.BinConsumer = ByteStreamConsumer()
	api.JSONConsumer = runtime.JSONConsumer()
	api.TarConsumer = ByteStreamConsumer()

	// FIXME: after updated go-openapi/runtime vendor code, revert ByteStreamProducer() back to runtime.ByteStreamProducer()
	api.BinProducer = ByteStreamProducer()
	api.JSONProducer = runtime.JSONProducer()
	api.TarProducer = ByteStreamProducer()
	api.TxtProducer = runtime.TextProducer()

	handlerCtx := &handlers.HandlerContext{
		Session: sess,
	}
	for _, handler := range portlayerhandlers {
		handler.Configure(api, handlerCtx)
	}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

func StopAPIServers() {
	for _, s := range apiServers {
		s.Stop(stopTimeout)
	}
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix"
func configureServer(s *graceful.Server, scheme string) {
	s.NoSignalHandling = true
	s.Timeout = stopTimeout
	apiServers = append(apiServers, s)
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return handler
}

// FIXME: to avoid update go-openapi/runtime vendor code at this time, write our own
// ByteStreamConsumer to read back encoded json format error message
func ByteStreamConsumer() runtime.Consumer {
	wrapped := runtime.ByteStreamConsumer()
	return runtime.ConsumerFunc(func(reader io.Reader, data interface{}) error {
		if reader == nil {
			return fmt.Errorf("ByteStreamConsumer requires a reader") // early exit
		}

		if er, ok := data.(*models.Error); ok {
			dec := json.NewDecoder(reader)
			return dec.Decode(er)
		}

		return wrapped.Consume(reader, data)
	})
}

// FIXME: to avoid update go-openapi/runtime vendor code at this time, write our own
// ByteStreamProducer to encode error to json string
func ByteStreamProducer() runtime.Producer {
	wrapped := runtime.ByteStreamProducer()
	return runtime.ProducerFunc(func(writer io.Writer, data interface{}) error {
		if writer == nil {
			return fmt.Errorf("ByteStreamProducer requires a writer") // early exit
		}

		if er, ok := data.(*models.Error); ok {
			enc := json.NewEncoder(writer)
			return enc.Encode(er)
		}

		return wrapped.Produce(writer, data)
	})
}
