package aci

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/api"
)

// DeleteContainerGroup deletes an Azure Container Instance in the provided
// resource group with the given container group name.
// From: https://docs.microsoft.com/en-us/rest/api/container-instances/containergroups/delete
func (c *Client) DeleteContainerGroup(resourceGroup, containerGroupName string) error {
	urlParams := url.Values{
		"api-version": []string{apiVersion},
	}

	// Create the url.
	uri := api.ResolveRelative(c.auth.ResourceManagerEndpoint, containerGroupURLPath)
	uri += "?" + url.Values(urlParams).Encode()

	// Create the request.
	req, err := http.NewRequest("DELETE", uri, nil)
	if err != nil {
		return fmt.Errorf("Creating delete container group uri request failed: %v", err)
	}

	// Add the parameters to the url.
	if err := api.ExpandURL(req.URL, map[string]string{
		"subscriptionId":     c.auth.SubscriptionID,
		"resourceGroup":      resourceGroup,
		"containerGroupName": containerGroupName,
	}); err != nil {
		return fmt.Errorf("Expanding URL with parameters failed: %v", err)
	}

	// Send the request.
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("Sending delete container group request failed: %v", err)
	}
	defer resp.Body.Close()

	if err := api.CheckResponse(resp); err != nil {
		return err
	}

	// 204 No Content means the specified container group was not found.
	if resp.StatusCode == http.StatusNoContent {
		return fmt.Errorf("Container group with name %q was not found", containerGroupName)
	}

	return nil
}
