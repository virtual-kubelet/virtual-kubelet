package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/filters"
)

// ServiceCreate creates a service in the Hyper.sh.
func (cli *Client) ServiceCreate(ctx context.Context, sv types.Service) (types.Service, error) {
	var service types.Service
	resp, err := cli.post(ctx, "/services/create", nil, sv, nil)
	if err != nil {
		return service, err
	}
	err = json.NewDecoder(resp.body).Decode(&service)
	ensureReaderClosed(resp)
	return service, err
}

// ServiceUpdate updates a service in the Hyper.sh.
func (cli *Client) ServiceUpdate(ctx context.Context, name string, opts types.ServiceUpdate) (types.Service, error) {
	var service types.Service
	resp, err := cli.post(ctx, "/services/"+name+"/update", nil, opts, nil)
	if err != nil {
		return service, err
	}
	err = json.NewDecoder(resp.body).Decode(&service)
	ensureReaderClosed(resp)
	return service, err
}

// ServiceDelete removes a service from the Hyper.sh.
func (cli *Client) ServiceDelete(ctx context.Context, id string, keep bool) error {
	v := url.Values{}
	v.Set("keey", "yes")
	resp, err := cli.delete(ctx, "/services/"+id, v, nil)
	ensureReaderClosed(resp)
	return err
}

// ServiceList returns the services configured in the docker host.
func (cli *Client) ServiceList(ctx context.Context, opts types.ServiceListOptions) ([]types.Service, error) {
	var services = []types.Service{}
	query := url.Values{}

	if opts.Filters.Len() > 0 {
		filterJSON, err := filters.ToParamWithVersion(cli.version, opts.Filters)
		if err != nil {
			return services, err
		}
		query.Set("filters", filterJSON)
	}
	resp, err := cli.get(ctx, "/services", query, nil)
	if err != nil {
		return services, err
	}

	err = json.NewDecoder(resp.body).Decode(&services)
	ensureReaderClosed(resp)
	return services, err
}

// ServiceInspect returns the information about a specific service in the docker host.
func (cli *Client) ServiceInspect(ctx context.Context, serviceID string) (types.Service, error) {
	service, _, err := cli.ServiceInspectWithRaw(ctx, serviceID)
	return service, err
}

// ServiceInspectWithRaw returns the information about a specific service in the docker host and it's raw representation
func (cli *Client) ServiceInspectWithRaw(ctx context.Context, serviceID string) (types.Service, []byte, error) {
	var service types.Service
	resp, err := cli.get(ctx, "/services/"+serviceID, nil, nil)
	if err != nil {
		if resp.statusCode == http.StatusNotFound {
			return service, nil, serviceNotFoundError{serviceID}
		}
		return service, nil, err
	}
	defer ensureReaderClosed(resp)

	body, err := ioutil.ReadAll(resp.body)
	if err != nil {
		return service, nil, err
	}
	rdr := bytes.NewReader(body)
	err = json.NewDecoder(rdr).Decode(&service)
	return service, body, err
}
