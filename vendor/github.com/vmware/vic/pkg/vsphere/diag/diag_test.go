// Copyright 2016 VMware, Inc. All Rights Reserved.
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
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/trace"
)

func TestCheckAPIAvailability(t *testing.T) {
	assert.Equal(t, VCStatusErrorQuery, CheckAPIAvailability("http://127.0.0.1:65535"))
	assert.Equal(t, VCStatusErrorQuery, CheckAPIAvailability("http://127.0.0.1:65536"))
}

func TestCheckAPIAvailabilityQueryWithGetterError(t *testing.T) {
	op := trace.NewOperation(context.Background(), "test")
	f := func(s string) (*http.Response, error) { return nil, errors.New("wrong query") }
	code := queryAPI(op, f, "testurl")
	assert.Equal(t, VCStatusErrorQuery, code)
}

type readerWithError struct {
	err  error
	data *bytes.Reader
}

func (r *readerWithError) Read(b []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.data.Read(b)
}

func (r *readerWithError) Close() error {
	return r.err
}

func TestCheckAPIAvailabilityQueryReadError(t *testing.T) {
	op := trace.NewOperation(context.Background(), "test")
	f := func(s string) (*http.Response, error) {
		hr := &http.Response{
			Body: &readerWithError{
				err: errors.New("read error happened"),
			},
		}
		return hr, nil
	}
	code := queryAPI(op, f, "testurl")
	assert.Equal(t, VCStatusErrorResponse, code)
}

func TestCheckAPIAvailabilityQueryIncorrectDataType(t *testing.T) {
	op := trace.NewOperation(context.Background(), "test")
	f := func(s string) (*http.Response, error) {
		hr := &http.Response{
			Body: &readerWithError{
				data: bytes.NewReader([]byte("some data")),
			},
		}
		return hr, nil
	}
	code := queryAPI(op, f, "testurl")
	assert.Equal(t, VCStatusNotXML, code)
}

func TestCheckAPIAvailabilityQueryIncorrectData(t *testing.T) {
	op := trace.NewOperation(context.Background(), "test")
	f := func(s string) (*http.Response, error) {
		hr := &http.Response{
			Body: &readerWithError{
				data: bytes.NewReader([]byte("some data")),
			},
			Header: http.Header{"Content-Type": []string{"text/xml"}},
		}
		return hr, nil
	}
	code := queryAPI(op, f, "testurl")
	assert.Equal(t, VCStatusIncorrectResponse, code)
}

func TestCheckAPIAvailabilityQueryCorrectData(t *testing.T) {
	op := trace.NewOperation(context.Background(), "test")
	f := func(s string) (*http.Response, error) {
		hr := &http.Response{
			Body: &readerWithError{
				data: bytes.NewReader([]byte("some urn:vim25Service data")),
			},
			Header: http.Header{"Content-Type": []string{"text/xml"}},
		}
		return hr, nil
	}
	code := queryAPI(op, f, "testurl")
	assert.Equal(t, VCStatusOK, code)
}

func TestCheckAPIAvailabilityIncorrectDNSName(t *testing.T) {
	assert.Equal(t, VCStatusUnknownHost, CheckAPIAvailability("https://example.notexisting.domain"))
}
