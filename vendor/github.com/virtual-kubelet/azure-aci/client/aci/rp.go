package aci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/virtual-kubelet/azure-aci/client/api"
)

const (
	resourceProviderURLPath    = "providers/Microsoft.ContainerInstance"
	resourceProviderAPIVersion = "2018-02-01"
)

// GetResourceProviderMetadata gets the ACI resource provider metadata
func (c *Client) GetResourceProviderMetadata(ctx context.Context) (*ResourceProviderMetadata, error) {
	manifest, err := c.getResourceProviderManifest(ctx)
	if err != nil {
		return nil, err
	}

	if manifest == nil {
		return nil, fmt.Errorf("The resource provider manifest is empty")
	}

	if manifest.Metadata == nil {
		return nil, fmt.Errorf("The resource provider metadata is empty")
	}

	return manifest.Metadata, nil
}

func (c *Client) getResourceProviderManifest(ctx context.Context) (*ResourceProviderManifest, error) {
	urlParams := url.Values{
		"api-version": []string{resourceProviderAPIVersion},
		"$expand":     []string{"metadata"},
	}

	// Create the url.
	uri := api.ResolveRelative(c.auth.ResourceManagerEndpoint, resourceProviderURLPath)
	uri += "?" + url.Values(urlParams).Encode()

	// Create the request.
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, fmt.Errorf("Creating get resource provider manifest request failed: %v", err)
	}
	req = req.WithContext(ctx)

	// Send the request.
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Sending get resource provider manifest request failed: %v", err)
	}
	defer resp.Body.Close()

	// 200 (OK) is a success response.
	if err := api.CheckResponse(resp); err != nil {
		return nil, err
	}

	// Decode the body from the response.
	if resp.Body == nil {
		return nil, errors.New("Get resource provider manifest returned an empty body in the response")
	}
	var manifest ResourceProviderManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("Decoding get resource provider manifest response body failed: %v", err)
	}

	return &manifest, nil
}
