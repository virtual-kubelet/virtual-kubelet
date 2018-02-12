package resourcegroups

import (
	"fmt"
	"net/http"

	azure "github.com/virtual-kubelet/virtual-kubelet/providers/azure/client"
)

const (
	// BaseURI is the default URI used for compute services.
	BaseURI    = "https://management.azure.com"
	userAgent  = "virtual-kubelet/azure-arm-resourcegroups/2017-12-01"
	apiVersion = "2017-08-01"

	resourceGroupURLPath = "subscriptions/{{.subscriptionId}}/resourcegroups/{{.resourceGroupName}}"
)

// Client is a client for interacting with Azure resource groups.
//
// Clients should be reused instead of created as needed.
// The methods of Client are safe for concurrent use by multiple goroutines.
type Client struct {
	hc   *http.Client
	auth *azure.Authentication
}

// NewClient creates a new Azure resource groups client.
func NewClient(auth *azure.Authentication) (*Client, error) {
	if auth == nil {
		return nil, fmt.Errorf("Authentication is not supplied for the Azure client")
	}

	client, err := azure.NewClient(auth, BaseURI, userAgent)
	if err != nil {
		return nil, fmt.Errorf("Creating Azure client failed: %v", err)
	}

	return &Client{hc: client.HTTPClient, auth: auth}, nil
}
