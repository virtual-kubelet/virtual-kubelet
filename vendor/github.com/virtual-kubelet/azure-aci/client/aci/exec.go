package aci

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/virtual-kubelet/azure-aci/client/api"
)

// TerminalSizeRequest is the terminal size request
type TerminalSizeRequest struct {
	Width  int
	Height int
}

// LaunchExec starts the exec command for a specified container instance in a specified resource group and container group.
// From: https://docs.microsoft.com/en-us/rest/api/container-instances/startcontainer/launchexec
func (c *Client) LaunchExec(resourceGroup, containerGroupName, containerName, command string, terminalSize TerminalSizeRequest) (ExecResponse, error) {
	urlParams := url.Values{
		"api-version": []string{apiVersion},
	}

	// Create the url to call Azure REST API
	uri := api.ResolveRelative(baseURI, containerExecURLPath)
	uri += "?" + url.Values(urlParams).Encode()

	var xc ExecRequest

	xc.Command = command
	xc.TerminalSize.Rows = terminalSize.Height
	xc.TerminalSize.Cols = terminalSize.Width

	var xcrsp ExecResponse
	xcrsp.Password = ""
	xcrsp.WebSocketURI = ""

	b := new(bytes.Buffer)

	if err := json.NewEncoder(b).Encode(xc); err != nil {
		return xcrsp, fmt.Errorf("Encoding create launch exec body request failed: %v", err)
	}

	req, err := http.NewRequest("POST", uri, b)
	if err != nil {
		return xcrsp, fmt.Errorf("Creating launch exec uri request failed: %v", err)
	}

	// Add the parameters to the url.
	if err := api.ExpandURL(req.URL, map[string]string{
		"subscriptionId":     c.auth.SubscriptionID,
		"resourceGroup":      resourceGroup,
		"containerGroupName": containerGroupName,
		"containerName":      containerName,
	}); err != nil {
		return xcrsp, fmt.Errorf("Expanding URL with parameters failed: %v", err)
	}

	// Send the request.
	resp, err := c.hc.Do(req)
	if err != nil {
		return xcrsp, fmt.Errorf("Sending launch exec request failed: %v", err)
	}
	defer resp.Body.Close()

	// 200 (OK) is a success response.
	if err := api.CheckResponse(resp); err != nil {
		return xcrsp, err
	}

	// Decode the body from the response.
	if resp.Body == nil {
		return xcrsp, errors.New("Create launch exec returned an empty body in the response")
	}

	if err := json.NewDecoder(resp.Body).Decode(&xcrsp); err != nil {
		return xcrsp, fmt.Errorf("Decoding create launch exec response body failed: %v", err)
	}

	return xcrsp, nil
}
