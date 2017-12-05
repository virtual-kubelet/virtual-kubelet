package client

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"context"
	"github.com/hyperhq/hyper-api/types"
)

func (cli *Client) ImageSaveTarFromDaemon(ctx context.Context, imageIDs []string) (io.ReadCloser, error) {
	query := url.Values{
		"names": imageIDs,
	}
	tr := &http.Transport{
		Dial: func(proto, addr string) (conn net.Conn, err error) {
			return net.Dial("unix", "/var/run/docker.sock")
		},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get("http://d/images/get?" + query.Encode())
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var dm struct {
			Message string `json:"message"`
		}
		errHead := "Error from local docker daemon: "
		if err := json.Unmarshal(data, &dm); err != nil {
			return nil, errors.New(errHead + string(data))
		}
		return nil, errors.New(errHead + dm.Message)
	}
	return resp.Body, nil
}

func (cli *Client) ImageDiff(ctx context.Context, allLayers [][]string, repoTags [][]string) (*types.ImageDiffResponse, error) {
	resp, err := cli.post(ctx, "/images/diff", nil, map[string]interface{}{
		"layers":   allLayers,
		"repoTags": repoTags,
	}, nil)
	if err != nil {
		return nil, err
	}
	var diffRet types.ImageDiffResponse
	err = json.NewDecoder(resp.body).Decode(&diffRet)
	ensureReaderClosed(resp)
	return &diffRet, nil
}

func (cli *Client) ImageLoadLocal(ctx context.Context, quiet bool, size int64) (*types.HijackedResponse, error) {
	query := url.Values{}
	query.Add("file", "true")
	query.Add("quiet", strconv.FormatBool(quiet))
	headers := http.Header{}
	headers.Add("X-Hyper-Content-Length", strconv.FormatInt(size, 10))

	resp, err := cli.postHijacked(ctx, "/images/load", query, nil, headers)
	if err != nil {
		return nil, err
	}

	if resp.Resp != nil && resp.Resp.StatusCode != http.StatusSwitchingProtocols {
		data, err := ioutil.ReadAll(resp.Resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, errors.New("Error response from daemon: " + string(data))
	}

	return &resp, nil
}
