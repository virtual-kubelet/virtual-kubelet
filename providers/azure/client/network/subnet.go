package network

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-05-01/network"
	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/api"
)

const (
	subnetPath = "subscriptions/{{.subscriptionId}}/resourcegroups/{{.resourceGroupName}}/providers/Microsoft.Network/virtualNetworks/{{.vnetName}}/subnets/{{.subnetName}}"
)

// Subnet represents an Azure subnet
type Subnet struct {
	Name       string
	ID         string
	Properties *SubnetProperties
}

// SubnetProperties are the properties for a subne
type SubnetProperties struct {
	AddressPrefix string `json:"addressPrefix,omitempty"`

	// IPConfigurationProfiles and Delegations are new fields not available in the SDK yet
	IPConfigurationProfiles []SubnetIPConfigurationProfile `json:"ipConfigurationProfiles"`
	Delegations             []Delegation

	// copied from official go SDK, none of these are used here except to make sure we don't nil out some data on fetched objects.
	// NetworkSecurityGroup - The reference of the NetworkSecurityGroup resource.
	NetworkSecurityGroup *network.SecurityGroup `json:"networkSecurityGroup,omitempty"`
	// RouteTable - The reference of the RouteTable resource.
	RouteTable *network.RouteTable `json:"routeTable,omitempty"`
	// ServiceEndpoints - An array of service endpoints.
	ServiceEndpoints *[]network.ServiceEndpointPropertiesFormat `json:"serviceEndpoints,omitempty"`
	// IPConfigurations - Gets an array of references to the network interface IP configurations using subnet.
	IPConfigurations *[]network.IPConfiguration `json:"ipConfigurations,omitempty"`
	// ResourceNavigationLinks - Gets an array of references to the external resources using subnet.
	ResourceNavigationLinks *[]network.ResourceNavigationLink `json:"resourceNavigationLinks,omitempty"`
	// ProvisioningState - The provisioning state of the resource.
	ProvisioningState *string `json:"provisioningState,omitempty"`
}

// SubnetIPConfigurationProfile stores the ID for an assigned network profile
type SubnetIPConfigurationProfile struct {
	ID string
}

// Delegation stores the subnet delegation details
type Delegation struct {
	Name       string
	ID         string
	ETag       string
	Properties DelegationProperties
}

// DelegationProperties stores the properties for a delegation
type DelegationProperties struct {
	ServiceName string
	Actions     []string
}

// GetSubnet gets the subnet from the specified resourcegroup/vnet
func (c *Client) GetSubnet(resourceGroup, vnet, name string) (*Subnet, error) {
	urlParams := url.Values{
		"api-version": []string{apiVersion},
	}

	// Create the url.
	uri := api.ResolveRelative(baseURI, subnetPath)
	uri += "?" + url.Values(urlParams).Encode()

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, errors.Wrap(err, "creating subnet get uri request failed")
	}

	if err := api.ExpandURL(req.URL, map[string]string{
		"subscriptionId":    c.auth.SubscriptionID,
		"resourceGroupName": resourceGroup,
		"subnetName":        name,
		"vnetName":          vnet,
	}); err != nil {
		return nil, errors.Wrap(err, "expanding URL with parameters failed")
	}

	// Send the request.
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "sending subnet get request failed")
	}
	defer resp.Body.Close()

	// 200 (OK) is a success response.
	if err := api.CheckResponse(resp); err != nil {
		return nil, err
	}

	var subnet Subnet
	if err := json.NewDecoder(resp.Body).Decode(&subnet); err != nil {
		return nil, err
	}
	return &subnet, nil
}

// CreateOrUpdateSubnet creates a new or updates an existing subnet in the defined resourcegroup/vnet
func (c *Client) CreateOrUpdateSubnet(resourceGroup, vnet string, s *Subnet) (*Subnet, error) {
	urlParams := url.Values{
		"api-version": []string{apiVersion},
	}

	// Create the url.
	uri := api.ResolveRelative(baseURI, subnetPath)
	uri += "?" + url.Values(urlParams).Encode()

	// Create the request.
	b, err := json.Marshal(s)
	if err != nil {
		return nil, errors.Wrap(err, "marshallig networking profile failed")
	}

	req, err := http.NewRequest("PUT", uri, bytes.NewReader(b))
	if err != nil {
		return nil, errors.New("creating subnet create uri request failed")
	}

	// Add the parameters to the url.
	if err := api.ExpandURL(req.URL, map[string]string{
		"subscriptionId":    c.auth.SubscriptionID,
		"resourceGroupName": resourceGroup,
		"subnetName":        s.Name,
		"vnetName":          vnet,
	}); err != nil {
		return nil, errors.Wrap(err, "expanding URL with parameters failed")
	}

	// Send the request.
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "sending create subnet request failed")
	}
	defer resp.Body.Close()

	// 200 (OK) is a success response.
	if err := api.CheckResponse(resp); err != nil {
		return nil, err
	}

	var subnet Subnet
	if err := json.NewDecoder(resp.Body).Decode(&subnet); err != nil {
		return nil, err
	}
	return &subnet, nil
}
