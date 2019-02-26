package aci

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/pkg/errors"
	"github.com/virtual-kubelet/azure-aci/client/api"
)

// GetContainerGroupMetrics gets metrics for the provided container group
func (c *Client) GetContainerGroupMetrics(ctx context.Context, resourceGroup, containerGroup string, options MetricsRequest) (*ContainerGroupMetricsResult, error) {
	if len(options.Types) == 0 {
		return nil, errors.New("must provide metrics types to fetch")
	}
	if options.Start.After(options.End) || options.Start.Equal(options.End) && !options.Start.IsZero() {
		return nil, errors.Errorf("end parameter must be after start: start=%s, end=%s", options.Start, options.End)
	}

	var metricNames string
	for _, t := range options.Types {
		if len(metricNames) > 0 {
			metricNames += ","
		}
		metricNames += string(t)
	}

	var ag string
	for _, a := range options.Aggregations {
		if len(ag) > 0 {
			ag += ","
		}
		ag += string(a)
	}

	urlParams := url.Values{
		"api-version": []string{"2018-01-01"},
		"aggregation": []string{ag},
		"metricnames": []string{metricNames},
		"interval":    []string{"PT1M"}, // TODO: make configurable?
	}

	if options.Dimension != "" {
		urlParams.Add("$filter", options.Dimension)
	}

	if !options.Start.IsZero() || !options.End.IsZero() {
		urlParams.Add("timespan", path.Join(options.Start.Format(time.RFC3339), options.End.Format(time.RFC3339)))
	}

	// Create the url.
	uri := api.ResolveRelative(c.auth.ResourceManagerEndpoint, containerGroupMetricsURLPath)
	uri += "?" + url.Values(urlParams).Encode()

	// Create the request.
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, errors.Wrap(err, "creating get container group metrics uri request failed")
	}
	req = req.WithContext(ctx)

	// Add the parameters to the url.
	if err := api.ExpandURL(req.URL, map[string]string{
		"subscriptionId":     c.auth.SubscriptionID,
		"resourceGroup":      resourceGroup,
		"containerGroupName": containerGroup,
	}); err != nil {
		return nil, errors.Wrap(err, "expanding URL with parameters failed")
	}

	// Send the request.
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "sending get container group metrics request failed")
	}
	defer resp.Body.Close()

	// 200 (OK) is a success response.
	if err := api.CheckResponse(resp); err != nil {
		return nil, err
	}

	// Decode the body from the response.
	if resp.Body == nil {
		return nil, errors.New("container group metrics returned an empty body in the response")
	}
	var metrics ContainerGroupMetricsResult
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		return nil, errors.Wrap(err, "decoding get container group metrics response body failed")
	}

	return &metrics, nil
}
