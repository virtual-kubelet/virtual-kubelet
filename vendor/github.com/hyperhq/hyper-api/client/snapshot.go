package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/filters"
)

// SnapshotList returns the snapshots configured in the docker host.
func (cli *Client) SnapshotList(ctx context.Context, filter filters.Args) (types.SnapshotsListResponse, error) {
	var snapshots types.SnapshotsListResponse
	query := url.Values{}

	if filter.Len() > 0 {
		filterJSON, err := filters.ToParam(filter)
		if err != nil {
			return snapshots, err
		}
		query.Set("filters", filterJSON)
	}
	resp, err := cli.get(ctx, "/snapshots", query, nil)
	if err != nil {
		return snapshots, err
	}

	err = json.NewDecoder(resp.body).Decode(&snapshots)
	ensureReaderClosed(resp)
	return snapshots, err
}

// SnapshotInspect returns the information about a specific snapshot in the docker host.
func (cli *Client) SnapshotInspect(ctx context.Context, snapshotID string) (types.Snapshot, error) {
	var snapshot types.Snapshot
	resp, err := cli.get(ctx, "/snapshots/"+snapshotID, nil, nil)
	if err != nil {
		if resp.statusCode == http.StatusNotFound {
			return snapshot, snapshotNotFoundError{snapshotID}
		}
		return snapshot, err
	}
	err = json.NewDecoder(resp.body).Decode(&snapshot)
	ensureReaderClosed(resp)
	return snapshot, err
}

// SnapshotCreate creates a snapshot in the docker host.
func (cli *Client) SnapshotCreate(ctx context.Context, options types.SnapshotCreateRequest) (types.Snapshot, error) {
	var snapshot types.Snapshot
	v := url.Values{}
	v.Set("volume", options.Volume)
	v.Set("name", options.Name)
	if options.Force {
		v.Set("force", "true")
	}
	resp, err := cli.post(ctx, "/snapshots/create", v, options, nil)
	if err != nil {
		return snapshot, err
	}
	err = json.NewDecoder(resp.body).Decode(&snapshot)
	ensureReaderClosed(resp)
	return snapshot, err
}

// SnapshotRemove removes a snapshot from the docker host.
func (cli *Client) SnapshotRemove(ctx context.Context, snapshotID string) error {
	resp, err := cli.delete(ctx, "/snapshots/"+snapshotID, nil, nil)
	ensureReaderClosed(resp)
	return err
}
