package resourcegroups

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/api"
)

// ResourceGroupExists checks if an Azure resource group exists.
// From: https://docs.microsoft.com/en-us/rest/api/resources/resourcegroups/checkexistence
func (c *Client) ResourceGroupExists(resourceGroup string) (bool, error) {
	urlParams := url.Values{
		"api-version": []string{apiVersion},
	}

	// Create the url.
	uri := api.ResolveRelative(BaseURI, resourceGroupURLPath)
	uri += "?" + url.Values(urlParams).Encode()

	// Create the request.
	req, err := http.NewRequest("HEAD", uri, nil)
	if err != nil {
		return false, fmt.Errorf("Creating resource group exists uri request failed: %v", err)
	}

	// Add the parameters to the url.
	if err := api.ExpandURL(req.URL, map[string]string{
		"subscriptionId":    c.auth.SubscriptionID,
		"resourceGroupName": resourceGroup,
	}); err != nil {
		return false, fmt.Errorf("Expanding URL with parameters failed: %v", err)
	}

	// Send the request.
	resp, err := c.hc.Do(req)
	if err != nil {
		return false, fmt.Errorf("Sending resource group exists request failed: %v", err)
	}
	defer resp.Body.Close()

	// A 404 response means it does not exit.
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	// 204 (NoContent) and 404 are successful responses.
	if err := api.CheckResponse(resp); err != nil {
		return false, err
	}

	// A 204 status means it exists.
	if resp.StatusCode == http.StatusNoContent {
		return true, nil
	}

	// A 404 status means it does not exist.
	return false, nil
}
