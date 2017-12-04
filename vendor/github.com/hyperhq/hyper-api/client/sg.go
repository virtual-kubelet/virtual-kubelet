package client

import (
	"context"
	"encoding/json"
	"io"
	"net/url"

	"github.com/hyperhq/hyper-api/types"
)

func (cli *Client) SgCreate(ctx context.Context, name string, data io.Reader) error {
	var v = url.Values{}
	serverResp, err := cli.postRaw(ctx, "/sg/"+name, v, data, nil)
	if err != nil {
		return err
	}

	ensureReaderClosed(serverResp)
	return nil
}

func (cli *Client) SgRm(ctx context.Context, name string) error {
	var v = url.Values{}
	serverResp, err := cli.delete(ctx, "/sg/"+name, v, nil)
	if err != nil {
		return err
	}

	ensureReaderClosed(serverResp)
	return nil
}

func (cli *Client) SgUpdate(ctx context.Context, name string, data io.Reader) error {
	var v = url.Values{}
	serverResp, err := cli.putRaw(ctx, "/sg/"+name, v, data, nil)
	if err != nil {
		return err
	}

	ensureReaderClosed(serverResp)
	return nil
}

func (cli *Client) SgInspect(ctx context.Context, name string) (*types.SecurityGroup, error) {
	var v = url.Values{}
	serverResp, err := cli.get(ctx, "/sg/"+name, v, nil)
	if err != nil {
		return nil, err
	}

	var sg types.SecurityGroup
	err = json.NewDecoder(serverResp.body).Decode(&sg)
	ensureReaderClosed(serverResp)
	return &sg, nil
}

func (cli *Client) SgLs(ctx context.Context) ([]types.SecurityGroup, error) {
	var v = url.Values{}
	serverResp, err := cli.get(ctx, "/sg", v, nil)
	if err != nil {
		return nil, err
	}

	var sgs []types.SecurityGroup
	err = json.NewDecoder(serverResp.body).Decode(&sgs)
	ensureReaderClosed(serverResp)
	return sgs, nil
}
