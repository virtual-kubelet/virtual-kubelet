package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strconv"

	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/filters"
)

func newFuncEndpointRequest(region, method, subpath string, query url.Values, body io.Reader) (*http.Request, error) {
	endpoint := os.Getenv("HYPER_FUNC_ENDPOINT")
	if endpoint == "" {
		endpoint = region + ".hyperfunc.io"
	}
	apiURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	apiURL.Scheme = "https"
	apiURL.Path = path.Join(apiURL.Path, subpath)
	queryStr := query.Encode()
	if queryStr != "" {
		apiURL.RawQuery = queryStr
	}
	req, err := http.NewRequest(method, apiURL.String(), body)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func funcEndpointRequestHijack(req *http.Request) (net.Conn, error) {
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "tcp")
	conn, err := tls.Dial("tcp", req.URL.Host+":443", &tls.Config{})
	if err != nil {
		return nil, err
	}
	clientConn := httputil.NewClientConn(conn, nil)
	resp, err := clientConn.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 101 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("Error response from server: %s", bytes.TrimSpace(body))
	}
	respConn, _ := clientConn.Hijack()
	return respConn, nil
}

func funcEndpointRequest(req *http.Request) (*http.Response, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{},
	}}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	status := resp.StatusCode
	if status < 200 || status >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("Error response from server: %s", bytes.TrimSpace(body))
	}
	return resp, nil
}

func (cli *Client) FuncCreate(ctx context.Context, opts types.Func) (types.Func, error) {
	var fn types.Func
	_, _, err := cli.ImageInspectWithRaw(context.Background(), opts.Config.Image, false)
	if err != nil {
		return fn, err
	}
	resp, err := cli.post(ctx, "/funcs/create", nil, opts, nil)
	if err != nil {
		return fn, err
	}
	err = json.NewDecoder(resp.body).Decode(&fn)
	ensureReaderClosed(resp)
	return fn, err
}

func (cli *Client) FuncUpdate(ctx context.Context, name string, opts types.Func) (types.Func, error) {
	var fn types.Func
	resp, err := cli.put(ctx, "/funcs/"+name, nil, opts, nil)
	if err != nil {
		return fn, err
	}
	err = json.NewDecoder(resp.body).Decode(&fn)
	ensureReaderClosed(resp)
	return fn, err
}

func (cli *Client) FuncDelete(ctx context.Context, name string) error {
	resp, err := cli.delete(ctx, "/funcs/"+name, nil, nil)
	ensureReaderClosed(resp)
	return err
}

func (cli *Client) FuncList(ctx context.Context, opts types.FuncListOptions) ([]types.Func, error) {
	var fns = []types.Func{}
	query := url.Values{}

	if opts.Filters.Len() > 0 {
		filterJSON, err := filters.ToParamWithVersion(cli.version, opts.Filters)
		if err != nil {
			return fns, err
		}
		query.Set("filters", filterJSON)
	}
	resp, err := cli.get(ctx, "/funcs", query, nil)
	if err != nil {
		return fns, err
	}

	err = json.NewDecoder(resp.body).Decode(&fns)
	ensureReaderClosed(resp)
	return fns, err
}

func (cli *Client) FuncInspect(ctx context.Context, name string) (types.Func, error) {
	fn, _, err := cli.FuncInspectWithRaw(ctx, name)
	return fn, err
}

func (cli *Client) FuncInspectWithRaw(ctx context.Context, name string) (types.Func, []byte, error) {
	var fn types.Func
	resp, err := cli.get(ctx, "/funcs/"+name, nil, nil)
	if err != nil {
		if resp.statusCode == http.StatusNotFound {
			return fn, nil, funcNotFoundError{name}
		}
		return fn, nil, err
	}
	defer ensureReaderClosed(resp)

	body, err := ioutil.ReadAll(resp.body)
	if err != nil {
		return fn, nil, err
	}
	rdr := bytes.NewReader(body)
	err = json.NewDecoder(rdr).Decode(&fn)
	return fn, body, err
}

func (cli *Client) FuncInspectWithCallId(ctx context.Context, id string) (*types.Func, error) {
	var fn types.Func
	resp, err := cli.get(ctx, "/funcs/call/"+id, nil, nil)
	if err != nil {
		if resp.statusCode == http.StatusNotFound {
			return nil, funcCallNotFoundError{id}
		}
		return nil, err
	}
	defer ensureReaderClosed(resp)

	body, err := ioutil.ReadAll(resp.body)
	if err != nil {
		return nil, err
	}
	rdr := bytes.NewReader(body)
	err = json.NewDecoder(rdr).Decode(&fn)
	return &fn, err
}

func (cli *Client) FuncCall(ctx context.Context, region, name string, stdin io.Reader, sync bool) (io.ReadCloser, error) {
	fn, _, err := cli.FuncInspectWithRaw(ctx, name)
	if err != nil {
		return nil, err
	}
	subpath := ""
	if sync {
		subpath += "/sync"
	}
	req, err := newFuncEndpointRequest(region, "POST", path.Join("call", name, fn.UUID, subpath), nil, stdin)
	if err != nil {
		return nil, err
	}
	resp, err := funcEndpointRequest(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (cli *Client) FuncGet(ctx context.Context, region, callId string, wait bool) (io.ReadCloser, error) {
	fn, err := cli.FuncInspectWithCallId(ctx, callId)
	if err != nil {
		return nil, err
	}
	subpath := callId
	if wait {
		subpath += "/wait"
	}
	req, err := newFuncEndpointRequest(region, "GET", path.Join("output", fn.Name, fn.UUID, subpath), nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := funcEndpointRequest(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (cli *Client) FuncLogs(ctx context.Context, region, name, callId string, follow bool, tail string) (io.ReadCloser, error) {
	fn, _, err := cli.FuncInspectWithRaw(ctx, name)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	if callId != "" {
		query.Set("callid", callId)
	}
	if follow {
		query.Add("follow", strconv.FormatBool(follow))
	}
	if tail != "" {
		query.Add("tail", tail)
	}
	req, err := newFuncEndpointRequest(region, "GET", path.Join("logs", name, fn.UUID, ""), query, nil)
	if err != nil {
		return nil, err
	}
	if follow {
		conn, err := funcEndpointRequestHijack(req)
		if err != nil {
			return nil, err
		}
		return conn.(io.ReadCloser), nil
	}
	resp, err := funcEndpointRequest(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (cli *Client) FuncStatus(ctx context.Context, region, name string) (*types.FuncStatusResponse, error) {
	fn, _, err := cli.FuncInspectWithRaw(ctx, name)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("list", strconv.FormatBool(false))
	req, err := newFuncEndpointRequest(region, "GET", path.Join("status", name, fn.UUID), query, nil)
	if err != nil {
		return nil, err
	}

	resp, err := funcEndpointRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ret types.FuncStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}
