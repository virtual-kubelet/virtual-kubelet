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

package session

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"
	"time"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/vic/pkg/vsphere/test/env"
)

func TestSessionDefaults(t *testing.T) {
	ctx := context.Background()

	config := &Config{
		Service:  env.URL(t),
		Insecure: true,
	}

	session, err := NewSession(config).Create(ctx)
	if err != nil {
		eStr := err.Error()
		t.Logf("%+v", eStr)
		// FIXME: See comments below
		if strings.Contains(eStr, "resolves to multiple hosts") {
			t.SkipNow()
		}
		t.Logf("%+v", eStr)
		if _, ok := err.(*find.DefaultMultipleFoundError); !ok {
			t.Errorf(eStr)
		} else {
			t.SkipNow()
		}
	}
	if session != nil {
		defer session.Logout(ctx)
	}

	t.Logf("%+v", session)
}

func TestSession(t *testing.T) {
	ctx := context.Background()

	config := &Config{
		Service:        env.URL(t),
		Insecure:       true,
		Keepalive:      time.Duration(5) * time.Minute,
		DatacenterPath: "",
		DatastorePath:  "/ha-datacenter/datastore/*",
		HostPath:       "/ha-datacenter/host/*/*",
		PoolPath:       "/ha-datacenter/host/*/Resources",
	}

	session, err := NewSession(config).Create(ctx)
	if err != nil {
		eStr := err.Error()
		t.Logf("%+v", eStr)
		// FIXME: session.Create incorporates Populate which loses the type of any original error from vmomi
		// In the case where the test is run on a cluster with multiple hosts, find.MultipleFoundError
		// gets rolled up into a generic error in Populate. As such, the best we can do is just grep for the string, which is lame
		// The test shouldn't fail if it's run on a cluster with multiple hosts. However, it won't test for anything either.
		if strings.Contains(eStr, "resolves to multiple hosts") {
			t.SkipNow()
		}
		if _, ok := err.(*find.MultipleFoundError); !ok {
			t.Errorf(eStr)
		} else {
			t.SkipNow()
		}
	}

	if session != nil {
		defer session.Logout(ctx)

		t.Logf("Session: %+v", session)

		t.Logf("IsVC: %t", session.IsVC())
		t.Logf("IsVSAN: %t", session.IsVSAN(ctx))
	}
}

func TestFolder(t *testing.T) {
	ctx := context.Background()

	config := &Config{
		Service:        env.URL(t),
		Insecure:       true,
		Keepalive:      time.Duration(5) * time.Minute,
		DatacenterPath: "",
		DatastorePath:  "/ha-datacenter/datastore/*",
		HostPath:       "/ha-datacenter/host/*/*",
		PoolPath:       "/ha-datacenter/host/*/Resources",
	}

	session, err := NewSession(config).Create(ctx)
	if err != nil {
		eStr := err.Error()
		t.Logf("%+v", eStr)
		// FIXME: See comments above
		if strings.Contains(eStr, "resolves to multiple hosts") {
			t.SkipNow()
		}
		if _, ok := err.(*find.MultipleFoundError); !ok {
			t.Errorf(eStr)
		} else {
			t.SkipNow()
		}
	}

	if session != nil {
		defer session.Logout(ctx)

		if session.VMFolder == nil {
			t.Errorf("Get empty folder")
		}
	}
}

func TestConnect(t *testing.T) {
	ctx := context.Background()

	for _, model := range []*simulator.Model{simulator.ESX(), simulator.VPX()} {
		defer model.Remove()
		err := model.Create()
		if err != nil {
			t.Fatal(err)
		}

		model.Service.TLS = new(tls.Config)
		s := model.Service.NewServer()
		defer s.Close()

		config := &Config{
			Keepalive: time.Minute,
			Service:   s.URL.String(),
		}

		for _, thumbprint := range []string{"", s.CertificateInfo().ThumbprintSHA1} {
			u := *s.URL
			config.Service = u.String()
			config.Thumbprint = thumbprint

			_, err = NewSession(config).Connect(ctx)
			if thumbprint == "" {
				if err == nil {
					t.Error("expected x509.UnknownAuthorityError error")
				}
			} else {
				if err != nil {
					t.Error(err)
				}
			}

			u.User = nil
			config.Service = u.String()
			_, err = NewSession(config).Connect(ctx)
			if err == nil {
				t.Fatal("expected login error")
			}

			config.Service = ""
			_, err = NewSession(config).Connect(ctx)
			if err == nil {
				t.Fatal("expected URL parse error")
			}
		}
	}
}
