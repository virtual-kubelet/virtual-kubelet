// Copyright 2017 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tags

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"github.com/vmware/govmomi/vim25/soap"
)

const (
	RestPrefix = "/rest"
	loginURL   = "/com/vmware/cis/session"
)

type RestClient struct {
	mu       sync.Mutex
	host     string
	scheme   string
	endpoint *url.URL
	user     *url.Userinfo
	HTTP     *http.Client
	cookies  []*http.Cookie
}

func NewClient(u *url.URL, insecure bool, thumbprint string) *RestClient {
	endpoint := &url.URL{}
	*endpoint = *u
	Logger.Debugf("Create rest client")
	endpoint.Path = RestPrefix

	sc := soap.NewClient(endpoint, insecure)
	if thumbprint != "" {
		sc.SetThumbprint(endpoint.Host, thumbprint)
	}

	user := endpoint.User
	endpoint.User = nil

	return &RestClient{
		endpoint: endpoint,
		user:     user,
		host:     endpoint.Host,
		scheme:   endpoint.Scheme,
		HTTP:     &sc.Client,
	}
}

func (c *RestClient) encodeData(data interface{}) (*bytes.Buffer, error) {
	params := bytes.NewBuffer(nil)
	if data != nil {
		if err := json.NewEncoder(params).Encode(data); err != nil {
			return nil, errors.Wrap(err, "failed to encode json data")
		}
	}
	return params, nil
}

func (c *RestClient) call(ctx context.Context, method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, http.Header, int, error) {
	//	Logger.Debugf("%s: %s, headers: %+v", method, path, headers)
	params, err := c.encodeData(data)
	if err != nil {
		return nil, nil, -1, errors.Wrap(err, "call failed")
	}

	if data != nil {
		if headers == nil {
			headers = make(map[string][]string)
		}
		headers["Content-Type"] = []string{"application/json"}
	}

	body, hdr, statusCode, err := c.clientRequest(ctx, method, path, params, headers)
	if statusCode == http.StatusUnauthorized && strings.Contains(err.Error(), "This method requires authentication") {
		c.Login(ctx)
		Logger.Debugf("Rerun request after login")
		return c.clientRequest(ctx, method, path, params, headers)
	}

	return body, hdr, statusCode, errors.Wrap(err, "call failed")
}

func (c *RestClient) clientRequest(ctx context.Context, method, path string, in io.Reader, headers map[string][]string) (io.ReadCloser, http.Header, int, error) {
	expectedPayload := (method == "POST" || method == "PUT")
	if expectedPayload && in == nil {
		in = bytes.NewReader([]byte{})
	}

	req, err := c.newRequest(method, path, in)
	if err != nil {
		return nil, nil, -1, errors.Wrap(err, "failed to create request")
	}

	req = req.WithContext(ctx)
	c.mu.Lock()
	if c.cookies != nil {
		req.AddCookie(c.cookies[0])
	}
	c.mu.Unlock()

	if headers != nil {
		for k, v := range headers {
			req.Header[k] = v
		}
	}

	if expectedPayload && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	return c.handleResponse(resp, err)
}

func (c *RestClient) handleResponse(resp *http.Response, err error) (io.ReadCloser, http.Header, int, error) {
	statusCode := -1
	if resp != nil {
		statusCode = resp.StatusCode
	}
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, nil, statusCode, errors.Errorf("Cannot connect to endpoint %s. Is vCloud Suite API running on this server?", c.host)
		}
		return nil, nil, statusCode, errors.Wrap(err, "error occurred trying to connect")
	}

	if statusCode < http.StatusOK || statusCode >= http.StatusBadRequest {
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, nil, statusCode, errors.Wrap(err, "error reading response")
		}
		if len(body) == 0 {
			return nil, nil, statusCode, errors.Errorf("Error: request returned %s", http.StatusText(statusCode))
		}
		Logger.Debugf("Error response: %s", bytes.TrimSpace(body))
		return nil, nil, statusCode, errors.Errorf("Error response from vCloud Suite API: %s", bytes.TrimSpace(body))
	}

	return resp.Body, resp.Header, statusCode, nil
}

func (c *RestClient) Login(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	Logger.Debugf("Login to %s through rest API.", c.host)

	request, err := c.newRequest("POST", loginURL, nil)
	if err != nil {
		return errors.Wrap(err, "login failed")
	}
	if c.user != nil {
		password, _ := c.user.Password()
		request.SetBasicAuth(c.user.Username(), password)
	}
	resp, err := c.HTTP.Do(request)
	if err != nil {
		return errors.Wrap(err, "login failed")
	}
	if resp == nil {
		return errors.New("response is nil in Login")
	}
	if resp.StatusCode != http.StatusOK {
		// #nosec: Errors unhandled.
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		return errors.Errorf("Login failed: body: %s, status: %s", bytes.TrimSpace(body), resp.Status)
	}

	c.cookies = resp.Cookies()

	Logger.Debugf("Login succeeded")
	return nil
}

func (c *RestClient) newRequest(method, urlStr string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(method, c.endpoint.String()+urlStr, body)
}
