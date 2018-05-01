// Copyright 2017 VMware, Inc. All Rights Reserved.
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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	goruntime "runtime"

	"github.com/Sirupsen/logrus"
	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/swag"
	"github.com/rs/cors"
	"github.com/tylerb/graceful"

	"github.com/vmware/vic/lib/apiservers/service/models"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers"
	"github.com/vmware/vic/lib/apiservers/service/restapi/operations"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
)

// This file is safe to edit. Once it exists it will not be overwritten

//go:generate swagger generate server --target ../lib/apiservers/service --name  --spec ../lib/apiservers/service/swagger.json --exclude-main

var loggingOption = struct {
	Directory string `long:"log-directory" description:"the directory where vic-machine-server log is stored" default:"/var/log/vic-machine-server" env:"LOG_DIRECTORY"`
	Level     string `long:"log-level" description:"the minimum log level for vic-machine-server log messages" default:"debug" env:"LOG_LEVEL" choice:"debug" choice:"info" choice:"warning" choice:"error"`
}{}

// logger is a workaround used to pass the logger instance from configureAPI to setupGlobalMiddleware
var logger *logrus.Logger

func configureFlags(api *operations.VicMachineAPI) {
	// api.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{ ... }
	api.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{
		{
			ShortDescription: "Logging options",
			LongDescription:  "Specify a directory for storing vic-machine service log",
			Options:          &loggingOption,
		},
	}
}

func configureAPI(api *operations.VicMachineAPI) http.Handler {
	// configure the api here
	api.ServeError = errors.ServeError

	// configure logging to user specified directory
	logger = configureLogger()
	api.Logger = logger.Infof
	api.Logger("Starting Service. Version: %q", version.GetBuild().ShortVersion())

	api.JSONConsumer = runtime.JSONConsumer()

	api.JSONProducer = runtime.JSONProducer()

	api.TxtProducer = runtime.TextProducer()

	// Applies when the Authorization header is set with the Basic scheme
	api.BasicAuth = handlers.BasicAuth

	api.SessionAuth = handlers.SessionAuth

	// GET /container
	api.GetHandler = operations.GetHandlerFunc(func(params operations.GetParams) middleware.Responder {
		return middleware.NotImplemented("operation .Get has not yet been implemented")
	})

	// GET /container/hello
	api.GetHelloHandler = &handlers.HelloGet{}

	// GET /container/version
	api.GetVersionHandler = &handlers.VersionGet{}

	// POST /container/target/{target}
	api.PostTargetTargetHandler = operations.PostTargetTargetHandlerFunc(func(params operations.PostTargetTargetParams, principal interface{}) middleware.Responder {
		return middleware.NotImplemented("operation .PostTargetTarget has not yet been implemented")
	})

	// GET /container/target/{target}/vch
	api.GetTargetTargetVchHandler = &handlers.VCHListGet{}

	// POST /container/target/{target}/vch
	api.PostTargetTargetVchHandler = &handlers.VCHCreate{}

	// GET /container/target/{target}/vch/{vch-id}
	api.GetTargetTargetVchVchIDHandler = &handlers.VCHGet{}

	// GET /container/target/{target}/vch/{vch-id}/certificate
	api.GetTargetTargetVchVchIDCertificateHandler = &handlers.VCHCertGet{}

	// GET /container/target/{target}/vch/{vch-id}/log
	api.GetTargetTargetVchVchIDLogHandler = &handlers.VCHLogGet{}

	// GET /container/target/{target}/datacenter/{datacenter}/vch/{vch-id}/certificate
	api.GetTargetTargetDatacenterDatacenterVchVchIDCertificateHandler = &handlers.VCHDatacenterCertGet{}

	// GET /container/target/{target}/datacenter/{datacenter}/vch/{vch-id}/log
	api.GetTargetTargetDatacenterDatacenterVchVchIDLogHandler = &handlers.VCHDatacenterLogGet{}

	// PUT /container/target/{target}/vch/{vch-id}
	api.PutTargetTargetVchVchIDHandler = operations.PutTargetTargetVchVchIDHandlerFunc(func(params operations.PutTargetTargetVchVchIDParams, principal interface{}) middleware.Responder {
		return middleware.NotImplemented("operation .PutTargetTargetVchVchID has not yet been implemented")
	})

	// PATCH /container/target/{target}/vch/{vch-id}
	api.PatchTargetTargetVchVchIDHandler = operations.PatchTargetTargetVchVchIDHandlerFunc(func(params operations.PatchTargetTargetVchVchIDParams, principal interface{}) middleware.Responder {
		return middleware.NotImplemented("operation .PatchTargetTargetVchVchID has not yet been implemented")
	})

	// POST /container/target/{target}/vch/{vch-id}
	api.PostTargetTargetVchVchIDHandler = operations.PostTargetTargetVchVchIDHandlerFunc(func(params operations.PostTargetTargetVchVchIDParams, principal interface{}) middleware.Responder {
		return middleware.NotImplemented("operation .PostTargetTargetVchVchID has not yet been implemented")
	})

	// DELETE /container/target/{target}/vch/{vch-id}
	api.DeleteTargetTargetVchVchIDHandler = &handlers.VCHDelete{}

	// POST /container/target/{target}/datacenter/{datacenter}
	api.PostTargetTargetDatacenterDatacenterHandler = operations.PostTargetTargetDatacenterDatacenterHandlerFunc(func(params operations.PostTargetTargetDatacenterDatacenterParams, principal interface{}) middleware.Responder {
		return middleware.NotImplemented("operation .PostTargetTargetDatacenterDatacenter has not yet been implemented")
	})

	// GET /container/target/{target}/datacenter/{datacenter}/vch
	api.GetTargetTargetDatacenterDatacenterVchHandler = &handlers.VCHDatacenterListGet{}

	// POST /container/target/{target}/datacenter/{datacenter}/vch
	api.PostTargetTargetDatacenterDatacenterVchHandler = &handlers.VCHDatacenterCreate{}

	// GET /container/target/{target}/datacenter/{datacenter}/vch/{vch-id}
	api.GetTargetTargetDatacenterDatacenterVchVchIDHandler = &handlers.VCHDatacenterGet{}

	// PUT /container/target/{target}/datacenter/{datacenter}/vch/{vch-id}
	api.PutTargetTargetDatacenterDatacenterVchVchIDHandler = operations.PutTargetTargetDatacenterDatacenterVchVchIDHandlerFunc(func(params operations.PutTargetTargetDatacenterDatacenterVchVchIDParams, principal interface{}) middleware.Responder {
		return middleware.NotImplemented("operation .PutTargetTargetDatacenterDatacenterVchVchID has not yet been implemented")
	})

	// PATCH /container/target/{target}/datacenter/{datacenter}/vch/{vch-id}
	api.PatchTargetTargetDatacenterDatacenterVchVchIDHandler = operations.PatchTargetTargetDatacenterDatacenterVchVchIDHandlerFunc(func(params operations.PatchTargetTargetDatacenterDatacenterVchVchIDParams, principal interface{}) middleware.Responder {
		return middleware.NotImplemented("operation .PatchTargetTargetDatacenterDatacenterVchVchID has not yet been implemented")
	})

	// POST /container/target/{target}/datacenter/{datacenter}/vch/{vch-id}
	api.PostTargetTargetDatacenterDatacenterVchVchIDHandler = operations.PostTargetTargetDatacenterDatacenterVchVchIDHandlerFunc(func(params operations.PostTargetTargetDatacenterDatacenterVchVchIDParams, principal interface{}) middleware.Responder {
		return middleware.NotImplemented("operation .PostTargetTargetDatacenterDatacenterVchVchID has not yet been implemented")
	})

	// DELETE /container/target/{target}/datacenter/{datacenter}/vch/{vch-id}
	api.DeleteTargetTargetDatacenterDatacenterVchVchIDHandler = &handlers.VCHDatacenterDelete{}

	api.ServerShutdown = func() {}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix"
func configureServer(s *graceful.Server, scheme string) {
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	// These settings have security implications. These settings should not be changed without appropriate review.
	// For more information, see the relevant section of the design document:
	// https://github.com/vmware/vic/blob/7f575392df99642c5edd8f539a74fe9c89155b00/doc/design/vic-machine/service.md#cross-origin-requests--cross-site-request-forgery
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "User-Agent", "X-VMWARE-TICKET"},
		AllowedMethods:   []string{"HEAD", "GET", "POST", "PUT", "PATCH", "DELETE"},
		ExposedHeaders:   []string{"Content-Length"},
		AllowCredentials: false,
	})

	return addLogging(addPanicRecovery(c.Handler(handler)))
}

func addLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op := trace.NewOperation(r.Context(), "%s %s request to %s", r.Proto, r.Method, r.URL.Path)

		lr := r.WithContext(op)
		lw := NewLoggingResponseWriter(w)

		op.Infof("Request: %s %s %s", r.Method, r.URL.Path, r.Proto)

		status := "??? UNKNOWN"
		defer func() {
			op.Infof("Response: %s", status)
		}()

		next.ServeHTTP(lw, lr)

		status = fmt.Sprintf("%d %s", lw.status, http.StatusText(lw.status))
	})
}

func configureLogger() *logrus.Logger {
	l := trace.Logger

	if _, err := os.Stat(loggingOption.Directory); os.IsNotExist(err) {
		os.MkdirAll(loggingOption.Directory, 0700)
	}

	path := loggingOption.Directory + "/vic-machine-server.log"
	file, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		log.Fatalf("Failed to open log file %s: %s", path, err)
	}

	l.Out = file

	level, err := logrus.ParseLevel(loggingOption.Level)
	if err != nil {
		log.Printf("Error parsing log level %s: %s", loggingOption.Level, err)
		level = logrus.DebugLevel
	}
	l.Level = level

	// In case code uses the global logrus logger (it shouldn't):
	logrus.SetOutput(file)

	return l
}

// addPanicRecovery middleware logs the panic err message and stack trace, and returns a json http response
func addPanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				op := trace.FromContext(r.Context(), "Panic Recovery")
				buf := make([]byte, 4096)
				bytes := goruntime.Stack(buf, false)
				stack := string(buf[:bytes])

				op.Errorf("PANIC: %s\n%s", err, stack)

				w.WriteHeader(http.StatusInternalServerError)
				e := models.Error{Message: fmt.Sprint(err)}
				json.NewEncoder(w).Encode(e)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// Reference for LoggingResponseWriter struct:
// http://ndersson.me/post/capturing_status_code_in_net_http/

type LoggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func NewLoggingResponseWriter(w http.ResponseWriter) *LoggingResponseWriter {
	return &LoggingResponseWriter{
		ResponseWriter: w,
	}
}

func (w *LoggingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
