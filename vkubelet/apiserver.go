package vkubelet

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/virtual-kubelet/virtual-kubelet/vkubelet/certs"
	"k8s.io/kubernetes/pkg/kubelet/server/remotecommand"
)

var p Provider
var r mux.Router

func NotFound(w http.ResponseWriter, r *http.Request) {
	log.Printf("404 request not found. \n %v", mux.Vars(r))
	http.Error(w, "404 request not found", http.StatusNotFound)
}

func ApiserverStart(provider Provider) {
	p = provider
	certFilePath := os.Getenv("APISERVER_CERT_LOCATION")
	keyFilePath := os.Getenv("APISERVER_KEY_LOCATION")

	var err error
	
	log.Println(certFilePath)
	log.Println(keyFilePath)
	
	// check to see if both the cert and key are empty
	// if not, pass on to http.ListenandServeTLS and rely 
	// on that failure.
	// if both are empty, generate self signed certs.
	if certFilePath == "" && keyFilePath == "" {
		log.Println("TLS key pair not provided for HTTP listener, generating one for you.")
		log.Println("WARNING: generated key pair is not suitable for production use.")
		certFilePath, keyFilePath, err = certs.GenerateCertKeyPair()
		if err != nil {
			log.Fatalf("error generating certificate pair for HTTP listener: %s\n", err)
		}
	}

	port := os.Getenv("KUBELET_PORT")
	addr := fmt.Sprintf(":%s", port)

	r := mux.NewRouter()
	r.HandleFunc("/containerLogs/{namespace}/{pod}/{container}", ApiServerHandler).Methods("GET")
	r.HandleFunc("/exec/{namespace}/{pod}/{container}", ApiServerHandlerExec).Methods("POST")
	r.NotFoundHandler = http.HandlerFunc(NotFound)

	if err := http.ListenAndServeTLS(addr, certFilePath, keyFilePath, r); err != nil {
		log.Printf("error starting http listener: %s\n", err)
	}
}

func ApiServerHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	if len(vars) == 3 {
		namespace := vars["namespace"]
		pod := vars["pod"]
		container := vars["container"]
		tail := 10
		q := req.URL.Query()
		queryTail := q.Get("tailLines")
		if queryTail != "" {
			t, err := strconv.Atoi(queryTail)
			if err != nil {
				log.Println(err)
				io.WriteString(w, err.Error())
			} else {
				tail = t
			}
		}
		podsLogs, err := p.GetContainerLogs(namespace, pod, container, tail)
		if err != nil {
			log.Println(err)
			io.WriteString(w, err.Error())
		} else {
			io.WriteString(w, podsLogs)
		}
	} else {
		NotFound(w, req)
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
