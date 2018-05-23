package vkubelet

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
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
	port := os.Getenv("KUBELET_PORT")
	addr := fmt.Sprintf(":%s", port)

	r := mux.NewRouter()
	r.HandleFunc("/containerLogs/{namespace}/{pod}/{container}", ApiServerHandlerLogs).Methods("GET")
	r.HandleFunc("/exec/{namespace}/{pod}/{container}", ApiServerHandlerExec).Methods("POST")
	r.NotFoundHandler = http.HandlerFunc(NotFound)

	if err := http.ListenAndServeTLS(addr, certFilePath, keyFilePath, r); err != nil {
		log.Println(err)
	}
}

func ApiServerHandlerLogs(w http.ResponseWriter, req *http.Request) {
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
}
