package aci

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/api"
)

// GetContainerGroup gets an Azure Container Instance in the provided
// resource group with the given container group name.
// From: https://docs.microsoft.com/en-us/rest/api/container-instances/containergroups/get
func (c *Client) GetContainerGroup(resourceGroup, containerGroupName string) (*ContainerGroup, error, *int) {
	urlParams := url.Values{
		"api-version": []string{apiVersion},
	}

	// Create the url.
	uri := api.ResolveRelative(BaseURI, containerGroupURLPath)
	uri += "?" + url.Values(urlParams).Encode()

	// Create the request.
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, fmt.Errorf("Creating get container group uri request failed: %v", err), nil
	}

	// Add the parameters to the url.
	if err := api.ExpandURL(req.URL, map[string]string{
		"subscriptionId":     c.auth.SubscriptionID,
		"resourceGroup":      resourceGroup,
		"containerGroupName": containerGroupName,
	}); err != nil {
		return nil, fmt.Errorf("Expanding URL with parameters failed: %v", err), nil
	}

	// Send the request.
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Sending get container group request failed: %v", err), &resp.StatusCode
	}
	defer resp.Body.Close()

	// 200 (OK) is a success response.
	if err := api.CheckResponse(resp); err != nil {
		return nil, err, &resp.StatusCode
	}

	// Decode the body from the response.
	if resp.Body == nil {
		return nil, errors.New("Create container group returned an empty body in the response"), &resp.StatusCode
	}
	var cg ContainerGroup
	if err := json.NewDecoder(resp.Body).Decode(&cg); err != nil {
		return nil, fmt.Errorf("Decoding get container group response body failed: %v", err), &resp.StatusCode
	}

	return &cg, nil, &resp.StatusCode
}
