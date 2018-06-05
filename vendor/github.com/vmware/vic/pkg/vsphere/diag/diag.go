// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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

package diag

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vmware/vic/pkg/trace"
)

// StatusCodeFatalThreshold defines a threshold after which all codes can be treated as fatal.
const StatusCodeFatalThreshold = 64

const (
	// VCStatusOK vSphere API is available.
	VCStatusOK = 0
	// VCStatusInvalidURL Provided vSphere API URL is wrong.
	VCStatusInvalidURL = 64
	// VCStatusErrorQuery Error happened trying to query vSphere API
	VCStatusErrorQuery = 65
	// VCStatusErrorResponse Received response doesn't contain expected data.
	VCStatusErrorResponse = 66
	// VCStatusIncorrectResponse Received in case if returned data from server is different from expected.
	VCStatusIncorrectResponse = 67
	// VCStatusNotXML Received response is not XML
	VCStatusNotXML = 68
	// VCStatusUnknownHost is returned in case if DNS failed to resolve name.
	VCStatusUnknownHost = 69
	// VCStatusHostIsNotReachable
	VCStatusHostIsNotReachable = 70
)

// UserReadableVCAPITestDescription convert API test code into user readable text
func UserReadableVCAPITestDescription(code int) string {
	switch code {
	case VCStatusOK:
		return "vSphere API target responds as expected"
	case VCStatusInvalidURL:
		return "vSphere API target url is invalid"
	case VCStatusErrorQuery:
		return "vSphere API target failed to respond to the query"
	case VCStatusIncorrectResponse:
		return "vSphere API target returns unexpected response"
	case VCStatusErrorResponse:
		return "vSphere API target returns error"
	case VCStatusNotXML:
		return "vSphere API target returns non XML response"
	case VCStatusUnknownHost:
		return "vSphere API target can not be resolved from VCH"
	case VCStatusHostIsNotReachable:
		return "vSphere API target is out of reach. Wrong routing table?"
	default:
		return "vSphere API target test returned unknown code"
	}
}

// CheckAPIAvailability accesses vSphere API to ensure it is a correct end point that is up and running.
func CheckAPIAvailability(targetURL string) int {
	op := trace.NewOperation(context.Background(), "api test")
	errorCode := VCStatusErrorQuery

	u, err := url.Parse(targetURL)
	if err != nil {
		return VCStatusInvalidURL
	}

	u.Path = "/sdk/vimService.wsdl"
	apiURL := u.String()

	op.Debugf("Checking access to: %s", apiURL)

	for attempts := 5; errorCode != VCStatusOK && attempts > 0; attempts-- {

		// #nosec: TLS InsecureSkipVerify set true
		c := http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
			// Is 20 seconds enough to receive any response from vSphere target server?
			Timeout: time.Second * 20,
		}
		errorCode = queryAPI(op, c.Get, apiURL)
	}
	return errorCode
}

func queryAPI(op trace.Operation, getter func(string) (*http.Response, error), apiURL string) int {
	resp, err := getter(apiURL)
	if err != nil {
		errTxt := err.Error()
		op.Errorf("Query error: %s", err)
		if strings.Contains(errTxt, "no such host") {
			return VCStatusUnknownHost
		}
		if strings.Contains(errTxt, "no route to host") {
			return VCStatusHostIsNotReachable
		}
		if strings.Contains(errTxt, "host is down") {
			return VCStatusHostIsNotReachable
		}
		return VCStatusErrorQuery
	}

	data := make([]byte, 65636)
	n, err := io.ReadFull(resp.Body, data)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		op.Errorf("Query error: %s", err)
		return VCStatusErrorResponse
	}
	if n >= len(data) {
		// #nosec: Errors unhandled.
		io.Copy(ioutil.Discard, resp.Body)
	}
	// #nosec: Errors unhandled.
	resp.Body.Close()

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "text/xml") {
		op.Errorf("Unexpected content type %s, should be text/xml", contentType)
		op.Errorf("Response from the server: %s", string(data))
		return VCStatusNotXML
	}
	// we just want to make sure that response contains something familiar that we could
	// use as vSphere API marker.
	if !bytes.Contains(data, []byte("urn:vim25Service")) {
		op.Errorf("Server response doesn't contain 'urn:vim25Service': %s", string(data))
		return VCStatusIncorrectResponse
	}
	return VCStatusOK
}
