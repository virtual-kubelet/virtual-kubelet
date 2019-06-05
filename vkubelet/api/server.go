package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
)

// ServeMux defines an interface used to attach routes to an existing http
// serve mux.
// It is used to enable callers creating a new server to completely manage
// their own HTTP server while allowing us to attach the required routes to
// satisfy the Kubelet HTTP interfaces.
type ServeMux interface {
	Handle(path string, h http.Handler)
}

type PodHandlerConfig struct {
	RunInContainer   ContainerExecHandlerFunc
	GetContainerLogs ContainerLogsHandlerFunc
	GetPods          PodListerFunc
}

// PodHandler creates an http handler for interacting with pods/containers.
func PodHandler(p PodHandlerConfig, debug bool) http.Handler {
	r := mux.NewRouter()

	// This matches the behaviour in the reference kubelet
	r.StrictSlash(true)
	if debug {
		r.HandleFunc("/runningpods/", HandleRunningPods(p.GetPods)).Methods("GET")
	}
	r.HandleFunc("/containerLogs/{namespace}/{pod}/{container}", HandleContainerLogs(p.GetContainerLogs)).Methods("GET")
	r.HandleFunc("/exec/{namespace}/{pod}/{container}", HandleContainerExec(p.RunInContainer)).Methods("POST")
	r.NotFoundHandler = http.HandlerFunc(NotFound)
	return r
}

// PodStatsSummaryHandler creates an http handler for serving pod metrics.
//
// If the passed in handler func is nil this will create handlers which only
//  serves http.StatusNotImplemented
func PodStatsSummaryHandler(f PodStatsSummaryHandlerFunc) http.Handler {
	if f == nil {
		return http.HandlerFunc(NotImplemented)
	}

	r := mux.NewRouter()

	const summaryRoute = "/stats/summary"
	h := HandlePodStatsSummary(f)

	r.Handle(summaryRoute, ochttp.WithRouteTag(h, "PodStatsSummaryHandler")).Methods("GET")
	r.Handle(summaryRoute+"/", ochttp.WithRouteTag(h, "PodStatsSummaryHandler")).Methods("GET")

	r.NotFoundHandler = http.HandlerFunc(NotFound)
	return r
}

// AttachPodRoutes adds the http routes for pod stuff to the passed in serve mux.
//
// Callers should take care to namespace the serve mux as they see fit, however
// these routes get called by the Kubernetes API server.
func AttachPodRoutes(p PodHandlerConfig, mux ServeMux, debug bool) {
	mux.Handle("/", InstrumentHandler(PodHandler(p, debug)))
}

// PodMetricsConfig stores the handlers for pod metrics routes
// It is used by AttachPodMetrics.
//
// The main reason for this struct is in case of expansion we do not need to break
// the package level API.
type PodMetricsConfig struct {
	GetStatsSummary PodStatsSummaryHandlerFunc
}

// AttachPodMetricsRoutes adds the http routes for pod/node metrics to the passed in serve mux.
//
// Callers should take care to namespace the serve mux as they see fit, however
// these routes get called by the Kubernetes API server.
func AttachPodMetricsRoutes(p PodMetricsConfig, mux ServeMux) {
	mux.Handle("/", InstrumentHandler(HandlePodStatsSummary(p.GetStatsSummary)))
}

func instrumentRequest(r *http.Request) *http.Request {
	ctx := r.Context()
	logger := log.G(ctx).WithFields(log.Fields{
		"uri":  r.RequestURI,
		"vars": mux.Vars(r),
	})
	ctx = log.WithLogger(ctx, logger)

	return r.WithContext(ctx)
}

// InstrumentHandler wraps an http.Handler and injects instrumentation into the request context.
func InstrumentHandler(h http.Handler) http.Handler {
	instrumented := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req = instrumentRequest(req)
		h.ServeHTTP(w, req)
	})
	return &ochttp.Handler{
		Handler:     instrumented,
		Propagation: &b3.HTTPFormat{},
	}
}

// NotFound provides a handler for cases where the requested endpoint doesn't exist
func NotFound(w http.ResponseWriter, r *http.Request) {
	log.G(r.Context()).Debug("404 request not found")
	http.Error(w, "404 request not found", http.StatusNotFound)
}

// NotImplemented provides a handler for cases where a provider does not implement a given API
func NotImplemented(w http.ResponseWriter, r *http.Request) {
	log.G(r.Context()).Debug("501 not implemented")
	http.Error(w, "501 not implemented", http.StatusNotImplemented)
}
