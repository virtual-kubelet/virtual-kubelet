package vkubelet

import (
	"encoding/base64"
	"io/ioutil"
	"io"
	"log"
	"net/http"
	"os"
	//"k8s.io/api/core/v1"
)

func ApiServerStart() error {
	http.HandleFunc("/", HelloServer)
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

func HelloServer(w http.ResponseWriter, req *http.Request) {
	log.Println("handler called")
	///containerLogs/{namespace}/{pd}/{container}
	log.Println(req)
	io.WriteString(w, "ack!\n")
}