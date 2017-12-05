package client

import (
	"encoding/json"

	"context"
	"github.com/hyperhq/hyper-api/types"
)

// CheckpointList returns the volumes configured in the docker host.
func (cli *Client) CheckpointList(ctx context.Context, container string) ([]types.Checkpoint, error) {
	var checkpoints []types.Checkpoint

	resp, err := cli.get(ctx, "/containers/"+container+"/checkpoints", nil, nil)
	if err != nil {
		return checkpoints, err
	}

	err = json.NewDecoder(resp.body).Decode(&checkpoints)
	ensureReaderClosed(resp)
	return checkpoints, err
}
