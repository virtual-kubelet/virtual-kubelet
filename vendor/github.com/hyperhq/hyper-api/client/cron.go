package client

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	"context"
	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/filters"
)

// CronCreate creates a cron in the Hyper_.
func (cli *Client) CronCreate(ctx context.Context, name string, sv types.Cron) (types.Cron, error) {
	var cron types.Cron
	var r = url.Values{}
	r.Set("name", name)
	resp, err := cli.post(ctx, "/crons/create", r, sv, nil)
	if err != nil {
		return cron, err
	}
	err = json.NewDecoder(resp.body).Decode(&cron)
	ensureReaderClosed(resp)
	return cron, err
}

// CronDelete removes a cron from the Hyper_.
func (cli *Client) CronDelete(ctx context.Context, id string) error {
	v := url.Values{}
	resp, err := cli.delete(ctx, "/crons/"+id, v, nil)
	ensureReaderClosed(resp)
	return err
}

// CronList returns the crons configured in the docker host.
func (cli *Client) CronList(ctx context.Context, opts types.CronListOptions) ([]types.Cron, error) {
	var crons = []types.Cron{}
	query := url.Values{}

	if opts.Filters.Len() > 0 {
		filterJSON, err := filters.ToParamWithVersion(cli.version, opts.Filters)
		if err != nil {
			return crons, err
		}
		query.Set("filters", filterJSON)
	}
	resp, err := cli.get(ctx, "/crons", query, nil)
	if err != nil {
		return crons, err
	}

	err = json.NewDecoder(resp.body).Decode(&crons)
	ensureReaderClosed(resp)
	return crons, err
}

// CronInspect returns the information about a specific cron in the docker host.
func (cli *Client) CronInspect(ctx context.Context, cronID string) (types.Cron, error) {
	cron, _, err := cli.CronInspectWithRaw(ctx, cronID)
	return cron, err
}

// CronInspectWithRaw returns the information about a specific cron in the docker host and it's raw representation
func (cli *Client) CronInspectWithRaw(ctx context.Context, cronID string) (types.Cron, []byte, error) {
	var cron types.Cron
	resp, err := cli.get(ctx, "/crons/"+cronID, nil, nil)
	if err != nil {
		if resp.statusCode == http.StatusNotFound {
			return cron, nil, cronNotFoundError{cronID}
		}
		return cron, nil, err
	}
	defer ensureReaderClosed(resp)

	body, err := ioutil.ReadAll(resp.body)
	if err != nil {
		return cron, nil, err
	}
	rdr := bytes.NewReader(body)
	err = json.NewDecoder(rdr).Decode(&cron)
	return cron, body, err
}

// CronHistory
func (cli *Client) CronHistory(ctx context.Context, id, since, tail string) ([]types.Event, error) {
	var (
		es = []types.Event{}
		v  = url.Values{}
	)
	if since != "" {
		v.Set("since", since)
	}
	if tail != "" {
		v.Set("tail", tail)
	}
	resp, err := cli.get(ctx, "/crons/"+id+"/history", v, nil)
	if err != nil {
		return nil, err
	}
	defer ensureReaderClosed(resp)

	err = json.NewDecoder(resp.body).Decode(&es)
	return es, err
}
