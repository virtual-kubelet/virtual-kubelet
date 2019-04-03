package aci

import (
	"fmt"
	"net/http"

	"go.opencensus.io/plugin/ochttp/propagation/b3"

	"go.opencensus.io/plugin/ochttp"

	azure "github.com/virtual-kubelet/azure-aci/client"
)

const (
	// BaseURI is the default URI used for compute services.
	baseURI          = "https://management.azure.com"
	defaultUserAgent = "virtual-kubelet/azure-arm-aci/2018-10-01"
	apiVersion       = "2018-10-01"

	containerGroupURLPath                    = "subscriptions/{{.subscriptionId}}/resourceGroups/{{.resourceGroup}}/providers/Microsoft.ContainerInstance/containerGroups/{{.containerGroupName}}"
	containerGroupListURLPath                = "subscriptions/{{.subscriptionId}}/providers/Microsoft.ContainerInstance/containerGroups"
	containerGroupListByResourceGroupURLPath = "subscriptions/{{.subscriptionId}}/resourceGroups/{{.resourceGroup}}/providers/Microsoft.ContainerInstance/containerGroups"
	containerLogsURLPath                     = containerGroupURLPath + "/containers/{{.containerName}}/logs"
	containerExecURLPath                     = containerGroupURLPath + "/containers/{{.containerName}}/exec"
	containerGroupMetricsURLPath             = containerGroupURLPath + "/providers/microsoft.Insights/metrics"
)

// Client is a client for interacting with Azure Container Instances.
//
// Clients should be reused instead of created as needed.
// The methods of Client are safe for concurrent use by multiple goroutines.
type Client struct {
	hc   *http.Client
	auth *azure.Authentication
}

// NewClient creates a new Azure Container Instances client with extra user agent.
func NewClient(auth *azure.Authentication, extraUserAgent string) (*Client, error) {
	if auth == nil {
		return nil, fmt.Errorf("Authentication is not supplied for the Azure client")
	}

	userAgent := []string{defaultUserAgent}
	if extraUserAgent != "" {
		userAgent = append(userAgent, extraUserAgent)
	}

	client, err := azure.NewClient(auth, baseURI, userAgent)
	if err != nil {
		return nil, fmt.Errorf("Creating Azure client failed: %v", err)
	}
	hc := client.HTTPClient
	hc.Transport = &ochttp.Transport{
		Base:           hc.Transport,
		Propagation:    &b3.HTTPFormat{},
		NewClientTrace: ochttp.NewSpanAnnotatingClientTrace,
	}

	return &Client{hc: client.HTTPClient, auth: auth}, nil
}
