package vkubelet

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/vkubelet/api"
)

// KubeletServerStart starts the virtual kubelet HTTP server.
func KubeletServerStart(p Provider) {
	certFilePath := os.Getenv("APISERVER_CERT_LOCATION")
	keyFilePath := os.Getenv("APISERVER_KEY_LOCATION")
	port := os.Getenv("KUBELET_PORT")
	addr := fmt.Sprintf(":%s", port)

	r := mux.NewRouter()
	r.HandleFunc("/containerLogs/{namespace}/{pod}/{container}", api.PodLogsHandlerFunc(p)).Methods("GET")
	r.HandleFunc("/exec/{namespace}/{pod}/{container}", api.PodExecHandlerFunc(p)).Methods("POST")
	r.NotFoundHandler = http.HandlerFunc(NotFound)

	if err := http.ListenAndServeTLS(addr, certFilePath, keyFilePath, InstrumentHandler(r)); err != nil {
		log.G(context.TODO()).WithError(err).Error("error setting up http server")
	}
}

// MetricsServerStart starts an HTTP server on the provided addr for serving the kubelset summary stats API.
// TLS is never enabled on this endpoint.
func MetricsServerStart(p Provider, addr string) {
	r := mux.NewRouter()

	mp, ok := p.(PodMetricsProvider)
	if !ok {
		r.HandleFunc("/stats/summary", NotImplemented).Methods("GET")
		r.HandleFunc("/stats/summary/", NotImplemented).Methods("GET")
	} else {
		r.HandleFunc("/stats/summary", api.PodMetricsHandlerFunc(mp)).Methods("GET")
		r.HandleFunc("/stats/summary/", api.PodMetricsHandlerFunc(mp)).Methods("GET")
	}
	r.NotFoundHandler = http.HandlerFunc(NotFound)
	if err := http.ListenAndServe(addr, InstrumentHandler(r)); err != nil {
		log.G(context.TODO()).WithError(err).Error("Error starting http server")
	}
}

func instrumentRequest(r *http.Request) context.Context {
	ctx := r.Context()
	logger := log.G(ctx).WithFields(logrus.Fields{
		"uri":  r.RequestURI,
		"vars": mux.Vars(r),
	})
	return log.WithLogger(ctx, logger)
}

// InstrumentHandler wraps an http.Handler and injects instrumentation into the request context.
func InstrumentHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := instrumentRequest(req)
		req = req.WithContext(ctx)
		h.ServeHTTP(w, req)
	})
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
