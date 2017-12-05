package client

import "context"

// ContainerUpdate updates resources of a container
func (cli *Client) ContainerUpdate(ctx context.Context, containerID string, updateConfig interface{}) error {
	resp, err := cli.put(ctx, "/containers/"+containerID, nil, updateConfig, nil)
	ensureReaderClosed(resp)
	return err
}
