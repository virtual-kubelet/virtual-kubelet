package vkubelet

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/vkubelet/api"
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

// PodHandler creates an http handler for interacting with pods/containers.
func PodHandler(p providers.Provider) http.Handler {
	r := mux.NewRouter()

	r.HandleFunc("/containerLogs/{namespace}/{pod}/{container}", api.PodLogsHandlerFunc(p)).Methods("GET")
	r.HandleFunc("/exec/{namespace}/{pod}/{container}", api.PodExecHandlerFunc(p)).Methods("POST")
	r.NotFoundHandler = http.HandlerFunc(NotFound)
	return r
}

// MetricsSummaryHandler creates an http handler for serving pod metrics.
//
// If the passed in provider does not implement providers.PodMetricsProvider,
// it will create handlers that just serves http.StatusNotImplemented
func MetricsSummaryHandler(p providers.Provider) http.Handler {
	r := mux.NewRouter()

	const summaryRoute = "/stats/summary"
	var h http.HandlerFunc

	mp, ok := p.(providers.PodMetricsProvider)
	if !ok {
		h = NotImplemented
	} else {
		h = api.PodMetricsHandlerFunc(mp)
	}

	r.Handle(summaryRoute, ochttp.WithRouteTag(h, "PodStatsSummaryHandler")).Methods("GET")
	r.Handle(summaryRoute+"/", ochttp.WithRouteTag(h, "PodStatsSummaryHandler")).Methods("GET")

	r.NotFoundHandler = http.HandlerFunc(NotFound)
	return r
}

// AttachPodRoutes adds the http routes for pod stuff to the passed in serve mux.
//
// Callers should take care to namespace the serve mux as they see fit, however
// these routes get called by the Kubernetes API server.
func AttachPodRoutes(p providers.Provider, mux ServeMux) {
	mux.Handle("/", InstrumentHandler(PodHandler(p)))
}

// AttachMetricsRoutes adds the http routes for pod/node metrics to the passed in serve mux.
//
// Callers should take care to namespace the serve mux as they see fit, however
// these routes get called by the Kubernetes API server.
func AttachMetricsRoutes(p providers.Provider, mux ServeMux) {
	mux.Handle("/", InstrumentHandler(MetricsSummaryHandler(p)))
}

func instrumentRequest(r *http.Request) *http.Request {
	ctx := r.Context()
	logger := log.G(ctx).WithFields(logrus.Fields{
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
	logger := log.G(r.Context())
	log.Trace(logger, "404 request not found")
	http.Error(w, "404 request not found", http.StatusNotFound)
}

// NotImplemented provides a handler for cases where a provider does not implement a given API
func NotImplemented(w http.ResponseWriter, r *http.Request) {
	logger := log.G(r.Context())
	log.Trace(logger, "501 not implemented")
	http.Error(w, "501 not implemented", http.StatusNotImplemented)
}
