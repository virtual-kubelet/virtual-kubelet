package client

import (
	"encoding/json"
	"net/url"

	"context"
	"github.com/hyperhq/hyper-api/types"
)

// ImageHistory returns the changes in an image in history format.
func (cli *Client) ImageHistory(ctx context.Context, imageID string) ([]types.ImageHistory, error) {
	var history []types.ImageHistory
	serverResp, err := cli.get(ctx, "/images/"+imageID+"/history", url.Values{}, nil)
	if err != nil {
		return history, err
	}

	err = json.NewDecoder(serverResp.body).Decode(&history)
	ensureReaderClosed(serverResp)
	return history, err
}
