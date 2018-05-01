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

package optmanager

import (
	"context"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/vic/pkg/vsphere/test"
)

func TestQueryOptionValue(t *testing.T) {
	ctx := context.Background()

	model := simulator.VPX()
	defer model.Remove()
	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	server := model.Service.NewServer()
	defer server.Close()

	s, err := test.SessionWithVPX(ctx, server.URL.String())
	if err != nil {
		t.Fatal(err)
	}

	// Multiple value error
	optValue, err := QueryOptionValue(ctx, s, "")
	if err == nil {
		t.Fatal("expected multiple value error")
	}

	// Invalid option
	optValue, err = QueryOptionValue(ctx, s, "foo-bar")
	if err == nil {
		t.Fatal("expected invalid query error")
	}

	// Valid option
	adminOptKey := "config.vpxd.sso.default.admin"
	adminOptVal := "Administrator@vsphere.local"
	optValue, err = QueryOptionValue(ctx, s, "config.vpxd.sso.default.admin")
	if err != nil {
		t.Fatalf("expected nil error, got: %s", err)
	}
	if optValue != adminOptVal {
		t.Fatalf("expected value %s for query %q, got: %s", adminOptVal, adminOptKey, optValue)
	}
}
