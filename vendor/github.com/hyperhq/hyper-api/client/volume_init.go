package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"context"

	"github.com/hyperhq/hyper-api/types"
)

// VolumeInitialize initializes a volume in the docker host.
func (cli *Client) VolumeInitialize(ctx context.Context, options types.VolumesInitializeRequest) (types.VolumesInitializeResponse, error) {
	var volResp types.VolumesInitializeResponse
	resp, err := cli.post(ctx, "/volumes/initialize", nil, options, nil)
	if err != nil {
		return types.VolumesInitializeResponse{}, err
	}
	err = json.NewDecoder(resp.body).Decode(&volResp)
	ensureReaderClosed(resp)
	return volResp, err
}

// VolumeUploadFinish notifies docker host of termination of a volume upload session
func (cli *Client) VolumeUploadFinish(ctx context.Context, session string) error {
	v := url.Values{}
	v.Set("session", session)
	resp, err := cli.put(ctx, "/volumes/uploadfinish", v, nil, nil)
	if err != nil {
		return err
	}
	ensureReaderClosed(resp)
	if resp.statusCode != http.StatusOK && resp.statusCode != http.StatusNoContent {
		return fmt.Errorf("Volume upload finish failed with %d", resp.statusCode)
	}
	return nil
}
