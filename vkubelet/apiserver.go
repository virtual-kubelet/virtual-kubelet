package vkubelet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/cpuguy83/strongerrors"
	"github.com/cpuguy83/strongerrors/status"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"k8s.io/kubernetes/pkg/kubelet/server/remotecommand"
)

func instrumentContext(r *http.Request) context.Context {
	ctx := r.Context()
	logger := log.G(ctx).WithFields(logrus.Fields{
		"uri":  r.RequestURI,
		"vars": mux.Vars(r),
	})
	return log.WithLogger(ctx, logger)
}

// NotFound provides a handler for cases where the requested endpoint doesn't exist
func NotFound(w http.ResponseWriter, r *http.Request) {
	logger := log.G(instrumentContext(r))
	log.Trace(logger, "404 request not found")
	http.Error(w, "404 request not found", http.StatusNotFound)
}

// NotImplemented provides a handler for cases where a provider does not implement a given API
func NotImplemented(w http.ResponseWriter, r *http.Request) {
	logger := log.G(instrumentContext(r))
	log.Trace(logger, "501 not implemented")
	http.Error(w, "501 not implemented", http.StatusNotImplemented)
}

type handlerFunc func(http.ResponseWriter, *http.Request) error

// InstrumentHandler wraps an http.Handler and injects instrumentation into the request context.
func InstrumentHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := instrumentContext(req)
		req = req.WithContext(ctx)
		h.ServeHTTP(w, req)
	})
}

func handleError(f handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		err := f(w, req)
		if err == nil {
			return
		}

		code, _ := status.HTTPCode(err)
		w.WriteHeader(code)
		io.WriteString(w, err.Error())
		logger := log.G(req.Context()).WithError(err).WithField("httpStatusCode", code)

		if code >= 500 {
			logger.Error("Internal server error on request")
		} else {
			log.Trace(logger, "Error on request")
		}
	}
}

// KubeletServerStart starts the virtual kubelet HTTP server.
func KubeletServerStart(p Provider) {
	certFilePath := os.Getenv("APISERVER_CERT_LOCATION")
	keyFilePath := os.Getenv("APISERVER_KEY_LOCATION")
	port := os.Getenv("KUBELET_PORT")
	addr := fmt.Sprintf(":%s", port)

	r := mux.NewRouter()
	r.HandleFunc("/containerLogs/{namespace}/{pod}/{container}", PodLogsHandlerFunc(p)).Methods("GET")
	r.HandleFunc("/exec/{namespace}/{pod}/{container}", PodExecHandlerFunc(p)).Methods("POST")
	r.NotFoundHandler = http.HandlerFunc(NotFound)

	if err := http.ListenAndServeTLS(addr, certFilePath, keyFilePath, InstrumentHandler(r)); err != nil {
		log.G(context.TODO()).WithError(err).Error("error setting up http server")
	}
}

// MetricsServerStart starts an HTTP server on the provided addr for serving the kubelset summary stats API.
// TLS is never enabled on this endpoint.
func MetricsServerStart(p Provider, addr string) {
	r := mux.NewRouter()

	mp, ok := p.(MetricsProvider)
	if !ok {
		r.HandleFunc("/stats/summary", NotImplemented).Methods("GET")
		r.HandleFunc("/stats/summary/", NotImplemented).Methods("GET")
	} else {
		r.HandleFunc("/stats/summary", PodMetricsHandlerFunc(mp)).Methods("GET")
		r.HandleFunc("/stats/summary/", PodMetricsHandlerFunc(mp)).Methods("GET")
	}
	r.NotFoundHandler = http.HandlerFunc(NotFound)
	if err := http.ListenAndServe(addr, InstrumentHandler(r)); err != nil {
		log.G(context.TODO()).WithError(err).Error("Error starting http server")
	}
}

// PodMetricsHandlerFunc makes an HTTP handler for implementing the kubelet summary stats endpoint
func PodMetricsHandlerFunc(mp MetricsProvider) http.HandlerFunc {
	return handleError(func(w http.ResponseWriter, req *http.Request) error {
		stats, err := mp.GetStatsSummary(req.Context())
		if err != nil {
			if errors.Cause(err) == context.Canceled {
				return strongerrors.Cancelled(err)
			}
			return strongerrors.Unknown(errors.Wrap(err, "error getting status from provider"))
		}

		b, err := json.Marshal(stats)
		if err != nil {
			return strongerrors.Unknown(errors.Wrap(err, "error marshalling stats"))
		}

		if _, err := w.Write(b); err != nil {
			return strongerrors.Unknown(errors.Wrap(err, "could not write to client"))
		}
		return nil
	})
}

// PodLogsHandlerFunc creates an http handler function from a provider to serve logs from a pod
func PodLogsHandlerFunc(p Provider) http.HandlerFunc {
	return handleError(func(w http.ResponseWriter, req *http.Request) error {
		vars := mux.Vars(req)
		if len(vars) != 3 {
			return strongerrors.NotFound(errors.New("not found"))
		}

		ctx := req.Context()

		namespace := vars["namespace"]
		pod := vars["pod"]
		container := vars["container"]
		tail := 10
		q := req.URL.Query()

		if queryTail := q.Get("tailLines"); queryTail != "" {
			t, err := strconv.Atoi(queryTail)
			if err != nil {
				return strongerrors.InvalidArgument(errors.Wrap(err, "could not parse \"tailLines\""))
			}
			tail = t
		}

		podsLogs, err := p.GetContainerLogs(ctx, namespace, pod, container, tail)
		if err != nil {
			return errors.Wrap(err, "error getting container logs?)")
		}

		if _, err := io.WriteString(w, podsLogs); err != nil {
			return strongerrors.Unknown(errors.Wrap(err, "error writing response to client"))
		}
		return nil
	})
}

// PodExecHandlerFunc makes an http handler func from a Provider which execs a command in a pod's container
func PodExecHandlerFunc(p Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)

		namespace := vars["namespace"]
		pod := vars["pod"]
		container := vars["container"]

		supportedStreamProtocols := strings.Split(req.Header.Get("X-Stream-Protocol-Version"), ",")

		q := req.URL.Query()
		command := q["command"]

		// TODO: tty flag causes remotecommand.createStreams to wait for the wrong number of streams
		streamOpts := &remotecommand.Options{
			Stdin:  true,
			Stdout: true,
			Stderr: true,
			TTY:    false,
		}

		idleTimeout := time.Second * 30
		streamCreationTimeout := time.Second * 30

		remotecommand.ServeExec(w, req, p, fmt.Sprintf("%s-%s", namespace, pod), "", container, command, streamOpts, idleTimeout, streamCreationTimeout, supportedStreamProtocols)
	}
}
