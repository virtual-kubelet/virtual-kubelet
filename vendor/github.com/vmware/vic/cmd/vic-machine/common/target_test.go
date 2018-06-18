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

package common

import (
	"context"
	"net/url"
	"testing"

	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/vic/pkg/trace"
)

func TestFlags(t *testing.T) {
	target := NewTarget()
	flags := target.TargetFlags()

	if len(flags) != 4 {
		t.Errorf("Wrong flag numbers")
	}
}

func TestProcess(t *testing.T) {
	op := trace.NewOperation(context.Background(), "TestProcess")

	passwd := "pass"
	url1, _ := soap.ParseURL("127.0.0.1")
	url2, _ := soap.ParseURL("root:@127.0.0.1")
	url3, _ := soap.ParseURL("line:password@127.0.0.1")
	url4, _ := soap.ParseURL("root:pass@127.0.0.1")
	result, _ := url.Parse("https://root:pass@127.0.0.1/sdk")
	passEmpty := ""
	result1, _ := url.Parse("https://root:@127.0.0.1/sdk")
	tests := []struct {
		URL      *url.URL
		User     string
		Password *string

		err    error
		result *url.URL
	}{
		{nil, "", nil, cli.NewExitError("--target argument must be specified", 1), nil},
		{nil, "root", nil, cli.NewExitError("--target argument must be specified", 1), nil},
		{nil, "root", &passwd, cli.NewExitError("--target argument must be specified", 1), nil},
		{url1, "root", &passwd, nil, result},
		{url4, "", nil, nil, result},
		{url3, "root", &passwd, nil, result},
		{url2, "", &passwd, nil, result},
		{url1, "root", &passEmpty, nil, result1},
	}

	for _, test := range tests {
		target := NewTarget()
		target.URL = test.URL
		target.User = test.User
		target.Password = test.Password
		if target.URL != nil {
			t.Logf("Before processing, url: %s", target.URL.String())
		}
		e := target.HasCredentials(op)
		if test.err != nil {
			if e == nil {
				t.Errorf("Empty error")
			}
			if e.Error() != test.err.Error() {
				t.Errorf("Unexpected error message: %s", e.Error())
			}
		} else if e != nil {
			t.Errorf("Unexpected error %s", e.Error())
		} else {
			if target.URL != test.URL {
				t.Errorf("unexpected result url: %s", target.URL.String())
			} else {
				t.Logf("result url: %s", target.URL.String())
			}
		}
	}
}
