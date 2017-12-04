package resourcegroups

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/api"
)

// DeleteResourceGroup deletes an Azure resource group.
// From: https://docs.microsoft.com/en-us/rest/api/resources/resourcegroups/delete
func (c *Client) DeleteResourceGroup(resourceGroup string) error {
	urlParams := url.Values{
		"api-version": []string{apiVersion},
	}

	// Create the url.
	uri := api.ResolveRelative(BaseURI, resourceGroupURLPath)
	uri += "?" + url.Values(urlParams).Encode()

	// Create the request.
	req, err := http.NewRequest("DELETE", uri, nil)
	if err != nil {
		return fmt.Errorf("Creating delete resource group uri request failed: %v", err)
	}

	// Add the parameters to the url.
	if err := api.ExpandURL(req.URL, map[string]string{
		"subscriptionId":    c.auth.SubscriptionID,
		"resourceGroupName": resourceGroup,
	}); err != nil {
		return fmt.Errorf("Expanding URL with parameters failed: %v", err)
	}

	// Send the request.
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("Sending delete resource group request failed: %v", err)
	}
	defer resp.Body.Close()

	// 200 (OK) and 202 (Accepted) are successful responses.
	if err := api.CheckResponse(resp); err != nil {
		return err
	}

	// 204 No Content means the specified resource group was not found.
	if resp.StatusCode == http.StatusNoContent {
		return fmt.Errorf("Resource group with name %q was not found", resourceGroup)
	}

	return nil
}
