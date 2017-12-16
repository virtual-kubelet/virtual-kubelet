package vkubelet

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)
var p Provider

func ApiserverStart(provider Provider) error {
	p = provider
	http.HandleFunc("/", ApiServerHandler)
	certFilePath := os.Getenv("APISERVER_CERT_LOCATION")
	keyFilePath := os.Getenv("APISERVER_KEY_LOCATION")
	err := http.ListenAndServeTLS(":10250", certFilePath, keyFilePath, nil)
	if err != nil {
		log.Fatal(err)
	}
	return err
}

func ApiServerHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		if strings.ContainsAny(req.RequestURI, "containerLogs" ) {
			reqParts := strings.Split(req.RequestURI, "/")
			if len(reqParts) == 5 {
				namespace := reqParts[2]
				pod := reqParts[3]
				container := reqParts[4]
				podsLogs, err := p.GetContainerLogs(namespace, pod, container)
				if err != nil {
					io.WriteString(w, err.Error())
					log.Fatal(err)
				} else {
					io.WriteString(w, podsLogs)
				}
			}
			
		}
	}
}