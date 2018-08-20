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
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"k8s.io/kubernetes/pkg/kubelet/server/remotecommand"
)

func loggingContext(r *http.Request) context.Context {
	ctx := r.Context()
	logger := log.G(ctx).WithFields(logrus.Fields{
		"uri":  r.RequestURI,
		"vars": mux.Vars(r),
	})
	return log.WithLogger(ctx, logger)
}

// NotFound provides a handler for cases where the requested endpoint doesn't exist
func NotFound(w http.ResponseWriter, r *http.Request) {
	logger := log.G(loggingContext(r))
	log.Trace(logger, "404 request not found")
	http.Error(w, "404 request not found", http.StatusNotFound)
}

// KubeletServer implements HTTP endpoints for serving kubelet API's
type KubeletServer struct {
	p Provider
}

// KubeletServertStart starts the virtual kubelet HTTP server.
func KubeletServerStart(p Provider) {
	certFilePath := os.Getenv("APISERVER_CERT_LOCATION")
	keyFilePath := os.Getenv("APISERVER_KEY_LOCATION")
	port := os.Getenv("KUBELET_PORT")
	addr := fmt.Sprintf(":%s", port)

	r := mux.NewRouter()
	s := &KubeletServer{p: p}
	r.HandleFunc("/containerLogs/{namespace}/{pod}/{container}", s.ApiServerHandler).Methods("GET")
	r.HandleFunc("/exec/{namespace}/{pod}/{container}", s.ApiServerHandlerExec).Methods("POST")
	r.NotFoundHandler = http.HandlerFunc(NotFound)

	if err := http.ListenAndServeTLS(addr, certFilePath, keyFilePath, r); err != nil {
		log.G(context.TODO()).WithError(err).Error("error setting up http server")
	}
}

// MetricsServerStart starts an HTTP server on the provided addr for serving the kubelset summary stats API.
// TLS is never enabled on this endpoint.
func MetricsServerStart(p Provider, addr string) {
	r := mux.NewRouter()
	s := &MetricsServer{p: p}
	r.HandleFunc("/stats/summary", s.MetricsSummaryHandler).Methods("GET")
	r.HandleFunc("/stats/summary/", s.MetricsSummaryHandler).Methods("GET")
	r.NotFoundHandler = http.HandlerFunc(NotFound)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.G(context.TODO()).WithError(err).Error("Error starting http server")
	}
}

// MetricsServer provides an HTTP endpopint for accessing pod metrics
type MetricsServer struct {
	p Provider
}

// MetricsSummaryHandler is an HTTP handler for implementing the kubelet summary stats endpoint
func (s *MetricsServer) MetricsSummaryHandler(w http.ResponseWriter, req *http.Request) {
	ctx := loggingContext(req)

	mp, ok := s.p.(MetricsProvider)
	if !ok {
		log.G(ctx).Debug("stats not implemented for provider")
		http.Error(w, "not implememnted", http.StatusNotImplemented)
		return
	}

	stats, err := mp.GetStatsSummary(req.Context())
	if err != nil {
		if errors.Cause(err) == context.Canceled {
			return
		}
		log.G(ctx).Error("Error getting stats from provider:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	b, err := json.Marshal(stats)
	if err != nil {
		log.G(ctx).WithError(err).Error("Could not marshal stats")
		http.Error(w, "could not marshal stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(b); err != nil {
		log.G(ctx).WithError(err).Debug("Could not write to client")
	}
}

func (s *KubeletServer) ApiServerHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	if len(vars) != 3 {
		NotFound(w, req)
		return
	}

	ctx := loggingContext(req)

	namespace := vars["namespace"]
	pod := vars["pod"]
	container := vars["container"]
	tail := 10
	q := req.URL.Query()

	if queryTail := q.Get("tailLines"); queryTail != "" {
		t, err := strconv.Atoi(queryTail)
		if err != nil {
			logger := log.G(context.TODO()).WithError(err)
			log.Trace(logger, "could not parse tailLines")
			http.Error(w, fmt.Sprintf("could not parse \"tailLines\": %v", err), http.StatusBadRequest)
			return
		}
		tail = t
	}

	podsLogs, err := s.p.GetContainerLogs(namespace, pod, container, tail)
	if err != nil {
		log.G(ctx).WithError(err).Error("error getting container logs")
		http.Error(w, fmt.Sprintf("error while getting container logs: %v", err), http.StatusInternalServerError)
		return
	}

	if _, err := io.WriteString(w, podsLogs); err != nil {
		log.G(ctx).WithError(err).Warn("error writing response to client")
	}
}

func (s *KubeletServer) ApiServerHandlerExec(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	namespace := vars["namespace"]
	pod := vars["pod"]
	container := vars["container"]

	supportedStreamProtocols := strings.Split(req.Header.Get("X-Stream-Protocol-Version"), ",")

	q := req.URL.Query()
	command := q["command"]

	// streamOpts := &remotecommand.Options{
	// 	Stdin:  (q.Get("input") == "1"),
	// 	Stdout: (q.Get("output") == "1"),
	// 	Stderr: (q.Get("error") == "1"),
	// 	TTY:    (q.Get("tty") == "1"),
	// }

	// TODO: tty flag causes remotecommand.createStreams to wait for the wrong number of streams
	streamOpts := &remotecommand.Options{
		Stdin:  true,
		Stdout: true,
		Stderr: true,
		TTY:    false,
	}

	idleTimeout := time.Second * 30
	streamCreationTimeout := time.Second * 30

	remotecommand.ServeExec(w, req, s.p, fmt.Sprintf("%s-%s", namespace, pod), "", container, command, streamOpts, idleTimeout, streamCreationTimeout, supportedStreamProtocols)
}
