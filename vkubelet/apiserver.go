package vkubelet

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"k8s.io/kubernetes/pkg/kubelet/server/remotecommand"
)

var p Provider
var r mux.Router

func loggingContext(r *http.Request) context.Context {
	ctx := r.Context()
	logger := log.G(ctx).WithFields(logrus.Fields{
		"uri":  r.RequestURI,
		"vars": mux.Vars(r),
	})
	return log.WithLogger(ctx, logger)
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	logger := log.G(loggingContext(r))
	log.Trace(logger, "404 request not found")
	http.Error(w, "404 request not found", http.StatusNotFound)
}

func ApiserverStart(provider Provider) {
	p = provider
	certFilePath := os.Getenv("APISERVER_CERT_LOCATION")
	keyFilePath := os.Getenv("APISERVER_KEY_LOCATION")
	port := os.Getenv("KUBELET_PORT")
	addr := fmt.Sprintf(":%s", port)

	r := mux.NewRouter()
	r.HandleFunc("/containerLogs/{namespace}/{pod}/{container}", ApiServerHandler).Methods("GET")
	r.HandleFunc("/exec/{namespace}/{pod}/{container}", ApiServerHandlerExec).Methods("POST")
	r.NotFoundHandler = http.HandlerFunc(NotFound)

	if err := http.ListenAndServeTLS(addr, certFilePath, keyFilePath, r); err != nil {
		log.G(context.TODO()).WithError(err).Error("error setting up http server")
	}
}

func ApiServerHandler(w http.ResponseWriter, req *http.Request) {
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

	podsLogs, err := p.GetContainerLogs(namespace, pod, container, tail)
	if err != nil {
		log.G(ctx).WithError(err).Error("error getting container logs")
		http.Error(w, fmt.Sprintf("error while getting container logs: %v", err), http.StatusInternalServerError)
		return
	}

	if _, err := io.WriteString(w, podsLogs); err != nil {
		log.G(ctx).WithError(err).Warn("error writing response to client")
	}
}

func ApiServerHandlerExec(w http.ResponseWriter, req *http.Request) {
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

	remotecommand.ServeExec(w, req, p, fmt.Sprintf("%s-%s", namespace, pod), "", container, command, streamOpts, idleTimeout, streamCreationTimeout, supportedStreamProtocols)
}
