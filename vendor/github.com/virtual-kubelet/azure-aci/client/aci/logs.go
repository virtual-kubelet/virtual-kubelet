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

// GetContainerLogs returns the logs from an Azure Container Instance
// in the provided resource group with the given container group name.
// From: https://docs.microsoft.com/en-us/rest/api/container-instances/ContainerLogs/List
func (c *Client) GetContainerLogs(ctx context.Context, resourceGroup, containerGroupName, containerName string, tail int) (*Logs, error) {
	urlParams := url.Values{
		"api-version": []string{apiVersion},
		"tail":        []string{fmt.Sprintf("%d", tail)},
	}

	// Create the url.
	uri := api.ResolveRelative(c.auth.ResourceManagerEndpoint, containerLogsURLPath)
	uri += "?" + url.Values(urlParams).Encode()

	// Create the request.
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, fmt.Errorf("Creating get container logs uri request failed: %v", err)
	}
	req = req.WithContext(ctx)

	// Add the parameters to the url.
	if err := api.ExpandURL(req.URL, map[string]string{
		"subscriptionId":     c.auth.SubscriptionID,
		"resourceGroup":      resourceGroup,
		"containerGroupName": containerGroupName,
		"containerName":      containerName,
	}); err != nil {
		return nil, fmt.Errorf("Expanding URL with parameters failed: %v", err)
	}

	// Send the request.
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Sending get container logs request failed: %v", err)
	}
	defer resp.Body.Close()

	// 200 (OK) is a success response.
	if err := api.CheckResponse(resp); err != nil {
		return nil, err
	}

	// Decode the body from the response.
	if resp.Body == nil {
		return nil, errors.New("Create container logs returned an empty body in the response")
	}
	var logs Logs
	if err := json.NewDecoder(resp.Body).Decode(&logs); err != nil {
		return nil, fmt.Errorf("Decoding get container logs response body failed: %v", err)
	}

	return &logs, nil
}
