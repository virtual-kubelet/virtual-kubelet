package vkubelet

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
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
	certValue64 := os.Getenv("APISERVER_CERT")
	keyValue64 := os.Getenv("APISERVER_KEY")
	certValue, err := base64.StdEncoding.DecodeString(certValue64)
	if err != nil {
		log.Fatal(err)
	}
	keyValue, err := base64.StdEncoding.DecodeString(keyValue64)
	if err != nil {
		log.Fatal(err)
	}
	cert := []byte(certValue)
 	key := []byte(keyValue)
 	certFilePath := "cert.pem"
 	keyFilePath := "key.pem"
 	err = ioutil.WriteFile(certFilePath, cert, 0644)
 	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile(keyFilePath, key, 0644)
 	if err != nil {
		log.Fatal(err)
	}

	err = http.ListenAndServeTLS(":10250", certFilePath, keyFilePath, nil)
	if err != nil {
		log.Fatal(err)
	}
	return nil
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
					fmt.Errorf("Error getting logs for pod '%s': %s", pod, err)
					io.WriteString(w, err.Error())
				} else {
					io.WriteString(w, podsLogs)
				}
			}
			
		}
	}
}