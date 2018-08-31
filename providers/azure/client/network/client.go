package network

import (
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-05-01/network"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	azure "github.com/virtual-kubelet/virtual-kubelet/providers/azure/client"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/api"
)

const (
	baseURI    = "https://management.azure.com"
	userAgent  = "virtual-kubelet/azure-arm-networking/2018-07-01"
	apiVersion = "2018-07-01"
)

// Client is a client for interacting with Azure networking
type Client struct {
	sc network.SubnetsClient
	hc *http.Client

	auth *azure.Authentication
}

// NewClient creates a new client for interacting with azure networking
func NewClient(azAuth *azure.Authentication) (*Client, error) {
	if azAuth == nil {
		return nil, fmt.Errorf("Authentication is not supplied for the Azure client")
	}

	client, err := azure.NewClient(azAuth, baseURI, userAgent)
	if err != nil {
		return nil, fmt.Errorf("Creating Azure client failed: %v", err)
	}

	authorizer, err := auth.NewClientCredentialsConfig(azAuth.ClientID, azAuth.ClientSecret, azAuth.TenantID).Authorizer()
	if err != nil {
		return nil, err
	}

	sc := network.NewSubnetsClient(azAuth.SubscriptionID)
	sc.Authorizer = authorizer

	return &Client{
		sc:   sc,
		hc:   client.HTTPClient,
		auth: azAuth,
	}, nil
}

// IsNotFound determines if the passed in error is a not found error from the API.
func IsNotFound(err error) bool {
	switch e := err.(type) {
	case nil:
		return false
	case *api.Error:
		return e.StatusCode == http.StatusNotFound
	default:
		return false
	}
}
