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

package dns

import (
	"net"
	"testing"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/lib/portlayer/network"
	"github.com/vmware/vic/pkg/trace"

	"context"

	mdns "github.com/miekg/dns"
)

var (
	Rtypes = []uint16{
		mdns.TypeA,
		mdns.TypeTXT,
		mdns.TypeAAAA,
	}

	Dnames = []string{
		"facebook.com.",
		"google.com.",
	}
)

func TestForwarding(t *testing.T) {
	t.Skipf("Failing with CI")

	log.SetLevel(log.PanicLevel)

	options.IP = "127.0.0.1"
	options.Port = 5354

	server := NewServer(options)
	if server != nil {
		server.Start()
	}

	c := new(mdns.Client)

	size := 1024
	for i := 0; i < size; i++ {
		for _, Δ := range Rtypes {
			for _, Θ := range Dnames {
				m := new(mdns.Msg)

				m.SetQuestion(Θ, Δ)

				r, _, err := c.Exchange(m, server.Addr())
				if err != nil || len(r.Answer) == 0 {
					t.Fatalf("Exchange failed: %s", err)
				}
			}
		}
	}

	n := len(Rtypes) * len(Dnames)
	if server.cache.Hits() != uint64(n*size-n) && server.cache.Misses() != uint64(size) {
		t.Fatalf("Cache hits %d misses %d", server.cache.Hits(), server.cache.Misses())
	}

	server.Stop()
	server.Wait()
}

func TestVIC(t *testing.T) {
	t.Skipf("Failing with CI")
	op := trace.NewOperation(context.Background(), "TestVIC")

	log.SetLevel(log.PanicLevel)

	options.IP = "127.0.0.1"
	options.Port = 5354

	// BEGIN - Context initialization
	var bridgeNetwork object.NetworkReference

	n := object.NewNetwork(nil, types.ManagedObjectReference{})
	n.InventoryPath = "testBridge"
	bridgeNetwork = n

	conf := &network.Configuration{
		Network: config.Network{
			BridgeNetwork: "lo",
			ContainerNetworks: map[string]*executor.ContainerNetwork{
				"lo": {
					Common: executor.Common{
						Name: "testBridge",
					},
					Type: constants.BridgeScopeType,
				},
			},
		},
		PortGroups: map[string]object.NetworkReference{
			"lo": bridgeNetwork,
		},
	}

	// initialize the context
	ctx, err := network.NewContext(conf, nil)
	if err != nil {
		t.Fatalf("%s", err)
	}

	// create the container
	con := exec.TestHandle("foo")
	ip := net.IPv4(172, 16, 0, 2)

	ctxOptions := &network.AddContainerOptions{
		Scope: "bridge",
		IP:    ip,
	}
	// add it
	err = ctx.AddContainer(con, ctxOptions)
	if err != nil {
		t.Fatalf("%s", err)
	}

	// bind it
	_, err = ctx.BindContainer(op, con)
	if err != nil {
		t.Fatalf("%s", err)
	}

	server := NewServer(options)
	if server != nil {
		server.Start()
	}
	// END - Context initialization

	m := new(mdns.Msg)
	m.SetQuestion("foo.", mdns.TypeA)

	c := new(mdns.Client)
	r, _, err := c.Exchange(m, server.Addr())
	if err != nil || len(r.Answer) == 0 {
		t.Fatalf("Exchange failed: %s", err)
	}

	m = new(mdns.Msg)
	m.SetQuestion("foo.bridge.", mdns.TypeA)

	r, _, err = c.Exchange(m, server.Addr())
	if err != nil || len(r.Answer) == 0 {
		t.Fatalf("Exchange failed: %s", err)
	}

	server.Stop()
	server.Wait()
}
