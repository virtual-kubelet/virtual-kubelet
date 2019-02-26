package network

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-08-01/network"
	"github.com/pkg/errors"
	"github.com/virtual-kubelet/azure-aci/client/api"
)

const (
	profilePath = "subscriptions/{{.subscriptionId}}/resourcegroups/{{.resourceGroupName}}/providers/Microsoft.Network/networkProfiles/{{.profileName}}"
)

var (
	defaultNicName      = "eth0"
	defaultIPConfigName = "ipconfigprofile1"
)

// NewNetworkProfile creates a new instance of network profile
func NewNetworkProfile(name, location, subnetID string) *network.Profile {
	p := network.Profile{
		Name:     &name,
		Location: &location,
		ProfilePropertiesFormat: &network.ProfilePropertiesFormat{
			ContainerNetworkInterfaceConfigurations: &[]network.ContainerNetworkInterfaceConfiguration{
				network.ContainerNetworkInterfaceConfiguration{
					Name: &defaultNicName,
					ContainerNetworkInterfaceConfigurationPropertiesFormat: &network.ContainerNetworkInterfaceConfigurationPropertiesFormat{
						IPConfigurations: &[]network.IPConfigurationProfile{
							network.IPConfigurationProfile{
								Name: &defaultIPConfigName,
								IPConfigurationProfilePropertiesFormat: &network.IPConfigurationProfilePropertiesFormat{
									Subnet: &network.Subnet{ID: &subnetID},
								},
							},
						},
					},
				},
			},
		},
	}

	return &p
}

// GetProfile gets the network profile with the provided name
func (c *Client) GetProfile(resourceGroup, name string) (*network.Profile, error) {
	urlParams := url.Values{
		"api-version": []string{apiVersion},
	}

	// Create the url.
	uri := api.ResolveRelative(baseURI, profilePath)
	uri += "?" + url.Values(urlParams).Encode()

	// Create the request.
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, errors.Wrap(err, "creating network profile get uri request failed")
	}

	// Add the parameters to the url.
	if err := api.ExpandURL(req.URL, map[string]string{
		"subscriptionId":    c.auth.SubscriptionID,
		"resourceGroupName": resourceGroup,
		"profileName":       name,
	}); err != nil {
		return nil, errors.Wrap(err, "expanding URL with parameters failed")
	}

	// Send the request.
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "sending get network profile request failed")
	}
	defer resp.Body.Close()

	// 200 (OK) is a success response.
	if err := api.CheckResponse(resp); err != nil {
		return nil, err
	}

	// Decode the body from the response.
	if resp.Body == nil {
		return nil, errors.New("get network profile returned an empty body in the response")
	}
	var p network.Profile
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, errors.Wrap(err, "decoding get network profile response body failed")
	}
	return &p, nil
}

// CreateOrUpdateProfile creates or updates an Azure network profile
func (c *Client) CreateOrUpdateProfile(resourceGroup string, p *network.Profile) (*network.Profile, error) {
	urlParams := url.Values{
		"api-version": []string{apiVersion},
	}

	// Create the url.
	uri := api.ResolveRelative(baseURI, profilePath)
	uri += "?" + url.Values(urlParams).Encode()

	// Create the request.
	b, err := json.Marshal(p)
	if err != nil {
		return nil, errors.Wrap(err, "marshalling networking profile failed")
	}

	req, err := http.NewRequest("PUT", uri, bytes.NewReader(b))
	if err != nil {
		return nil, errors.Wrap(err, "creating network profile create uri request failed")
	}

	// Add the parameters to the url.
	if err := api.ExpandURL(req.URL, map[string]string{
		"subscriptionId":    c.auth.SubscriptionID,
		"resourceGroupName": resourceGroup,
		"profileName":       *p.Name,
	}); err != nil {
		return nil, errors.Wrap(err, "expanding URL with parameters failed")
	}

	// Send the request.
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "sending get network profile request failed")
	}
	defer resp.Body.Close()

	// 200 (OK) is a success response.
	if err := api.CheckResponse(resp); err != nil {
		return nil, err
	}

	// Decode the body from the response.
	if resp.Body == nil {
		return nil, errors.New("create network profile returned an empty body in the response")
	}

	var profile network.Profile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, errors.Wrap(err, "decoding create network profile response body failed")
	}

	return &profile, nil
}
