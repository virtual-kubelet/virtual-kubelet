package huawei

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/mux"
	v1 "k8s.io/api/core/v1"
)

// CCIMock implements a CCI service mock server.
type CCIMock struct {
	server          *httptest.Server
	OnCreateProject func(*v1.Namespace) (int, interface{})
	OnCreatePod     func(*v1.Pod) (int, interface{})
	OnGetPods       func() (int, interface{})
	OnGetPod        func(string, string) (int, interface{})
}

// fakeSigner signature HWS meta
type fakeSigner struct {
	AppKey    string
	AppSecret string
	Region    string
	Service   string
}

// Sign set Authorization header
func (s *fakeSigner) Sign(r *http.Request) error {
	return nil
}

const (
	cciProjectRoute = "/api/v1/namespaces"
	cciPodsRoute    = cciProjectRoute + "/{namespaceID}/pods"
	cciPodRoute     = cciPodsRoute + "/{podID}"
)

// NewCCIMock creates a CCI service mock server.
func NewCCIMock() *CCIMock {
	mock := new(CCIMock)
	mock.start()

	return mock
}

// Start the CCI service mock service.
func (mock *CCIMock) start() {
	if mock.server != nil {
		return
	}

	router := mux.NewRouter()
	router.HandleFunc(
		cciProjectRoute,
		func(w http.ResponseWriter, r *http.Request) {
			var ns v1.Namespace
			if err := json.NewDecoder(r.Body).Decode(&ns); err != nil {
				panic(err)
			}

			if mock.OnCreatePod != nil {
				statusCode, response := mock.OnCreateProject(&ns)
				w.WriteHeader(statusCode)
				b := new(bytes.Buffer)
				json.NewEncoder(b).Encode(response)
				w.Write(b.Bytes())

				return
			}

			w.WriteHeader(http.StatusNotImplemented)
		}).Methods("PUT")

	router.HandleFunc(
		cciPodsRoute,
		func(w http.ResponseWriter, r *http.Request) {
			var pod v1.Pod
			if err := json.NewDecoder(r.Body).Decode(&pod); err != nil {
				panic(err)
			}

			if mock.OnCreatePod != nil {
				statusCode, response := mock.OnCreatePod(&pod)
				w.WriteHeader(statusCode)
				b := new(bytes.Buffer)
				json.NewEncoder(b).Encode(response)
				w.Write(b.Bytes())

				return
			}

			w.WriteHeader(http.StatusNotImplemented)
		}).Methods("PUT")

	router.HandleFunc(
		cciPodRoute,
		func(w http.ResponseWriter, r *http.Request) {
			namespace, _ := mux.Vars(r)["namespaceID"]
			podname, _ := mux.Vars(r)["podID"]

			if mock.OnGetPod != nil {
				statusCode, response := mock.OnGetPod(namespace, podname)
				w.WriteHeader(statusCode)
				b := new(bytes.Buffer)
				json.NewEncoder(b).Encode(response)
				w.Write(b.Bytes())

				return
			}

			w.WriteHeader(http.StatusNotImplemented)
		}).Methods("GET")

	router.HandleFunc(
		cciPodsRoute,
		func(w http.ResponseWriter, r *http.Request) {
			if mock.OnGetPods != nil {
				statusCode, response := mock.OnGetPods()
				w.WriteHeader(statusCode)
				b := new(bytes.Buffer)
				json.NewEncoder(b).Encode(response)
				w.Write(b.Bytes())

				return
			}

			w.WriteHeader(http.StatusNotImplemented)
		}).Methods("GET")

	mock.server = httptest.NewServer(router)
}

// GetServerURL returns the mock server URL.
func (mock *CCIMock) GetServerURL() string {
	if mock.server != nil {
		return mock.server.URL
	}

	panic("Mock server is not initialized.")
}

// Close terminates the CCI mock server.
func (mock *CCIMock) Close() {
	if mock.server != nil {
		mock.server.Close()
		mock.server = nil
	}
}
