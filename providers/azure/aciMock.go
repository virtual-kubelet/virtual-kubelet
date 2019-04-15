package azure

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/mux"
	"github.com/virtual-kubelet/azure-aci/client/aci"
)

// ACIMock implements a Azure Container Instance mock server.
type ACIMock struct {
	server               *httptest.Server
	OnCreate             func(string, string, string, *aci.ContainerGroup) (int, interface{})
	OnGetContainerGroups func(string, string) (int, interface{})
	OnGetContainerGroup  func(string, string, string) (int, interface{})
	OnGetRPManifest      func() (int, interface{})
}

const (
	containerGroupsRoute   = "/subscriptions/{subscriptionId}/resourceGroups/{resourceGroup}/providers/Microsoft.ContainerInstance/containerGroups"
	containerGroupRoute    = containerGroupsRoute + "/{containerGroup}"
	containerGroupLogRoute = containerGroupRoute + "/containers/{containerName}/logs"
	resourceProviderRoute  = "/providers/Microsoft.ContainerInstance"
)

// NewACIMock creates a new Azure Container Instance mock server.
func NewACIMock() *ACIMock {
	mock := new(ACIMock)
	mock.start()

	return mock
}

// Start the Azure Container Instance mock service.
func (mock *ACIMock) start() {
	if mock.server != nil {
		return
	}

	router := mux.NewRouter()
	router.HandleFunc(
		containerGroupRoute,
		func(w http.ResponseWriter, r *http.Request) {
			subscription, _ := mux.Vars(r)["subscriptionId"]
			resourceGroup, _ := mux.Vars(r)["resourceGroup"]
			containerGroup, _ := mux.Vars(r)["containerGroup"]

			var cg aci.ContainerGroup
			if err := json.NewDecoder(r.Body).Decode(&cg); err != nil {
				panic(err)
			}

			if mock.OnCreate != nil {
				statusCode, response := mock.OnCreate(subscription, resourceGroup, containerGroup, &cg)
				w.WriteHeader(statusCode)
				b := new(bytes.Buffer)
				json.NewEncoder(b).Encode(response)
				w.Write(b.Bytes())

				return
			}

			w.WriteHeader(http.StatusNotImplemented)
		}).Methods("PUT")

	router.HandleFunc(
		containerGroupRoute,
		func(w http.ResponseWriter, r *http.Request) {
			subscription, _ := mux.Vars(r)["subscriptionId"]
			resourceGroup, _ := mux.Vars(r)["resourceGroup"]
			containerGroup, _ := mux.Vars(r)["containerGroup"]

			if mock.OnGetContainerGroup != nil {
				statusCode, response := mock.OnGetContainerGroup(subscription, resourceGroup, containerGroup)
				w.WriteHeader(statusCode)
				b := new(bytes.Buffer)
				json.NewEncoder(b).Encode(response)
				w.Write(b.Bytes())

				return
			}

			w.WriteHeader(http.StatusNotImplemented)
		}).Methods("GET")

	router.HandleFunc(
		containerGroupsRoute,
		func(w http.ResponseWriter, r *http.Request) {
			subscription, _ := mux.Vars(r)["subscriptionId"]
			resourceGroup, _ := mux.Vars(r)["resourceGroup"]

			if mock.OnGetContainerGroups != nil {
				statusCode, response := mock.OnGetContainerGroups(subscription, resourceGroup)
				w.WriteHeader(statusCode)
				b := new(bytes.Buffer)
				json.NewEncoder(b).Encode(response)
				w.Write(b.Bytes())

				return
			}

			w.WriteHeader(http.StatusNotImplemented)
		}).Methods("GET")

	router.HandleFunc(
		resourceProviderRoute,
		func(w http.ResponseWriter, r *http.Request) {
			if mock.OnGetRPManifest != nil {
				statusCode, response := mock.OnGetRPManifest()
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
func (mock *ACIMock) GetServerURL() string {
	if mock.server != nil {
		return mock.server.URL
	}

	panic("Mock server is not initialized.")
}

// Close terminates the Azure Container Instance mock server.
func (mock *ACIMock) Close() {
	if mock.server != nil {
		mock.server.Close()
		mock.server = nil
	}
}
