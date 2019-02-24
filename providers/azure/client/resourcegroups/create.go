package resourcegroups

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/iofog/virtual-kubelet/providers/azure/client/api"
)

// CreateResourceGroup creates a new Azure resource group with the
// provided properties.
// From: https://docs.microsoft.com/en-us/rest/api/resources/resourcegroups/createorupdate
func (c *Client) CreateResourceGroup(resourceGroup string, properties Group) (*Group, error) {
	urlParams := url.Values{
		"api-version": []string{apiVersion},
	}

	// Create the url.
	uri := api.ResolveRelative(BaseURI, resourceGroupURLPath)
	uri += "?" + url.Values(urlParams).Encode()

	// Create the body for the request.
	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(properties); err != nil {
		return nil, fmt.Errorf("Encoding create resource group body request failed: %v", err)
	}

	// Create the request.
	req, err := http.NewRequest("PUT", uri, b)
	if err != nil {
		return nil, fmt.Errorf("Creating create/update resource group uri request failed: %v", err)
	}

	// Add the parameters to the url.
	if err := api.ExpandURL(req.URL, map[string]string{
		"subscriptionId":    c.auth.SubscriptionID,
		"resourceGroupName": resourceGroup,
	}); err != nil {
		return nil, fmt.Errorf("Expanding URL with parameters failed: %v", err)
	}

	// Send the request.
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Sending create resource group request failed: %v", err)
	}
	defer resp.Body.Close()

	// 200 (OK) and 201 (Created) are a successful responses.
	if err := api.CheckResponse(resp); err != nil {
		return nil, err
	}

	// Decode the body from the response.
	if resp.Body == nil {
		return nil, errors.New("Create resource group returned an empty body in the response")
	}
	var g Group
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return nil, fmt.Errorf("Decoding create resource group response body failed: %v", err)
	}

	return &g, nil
}
