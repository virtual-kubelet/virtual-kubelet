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

package handlers

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	cli "gopkg.in/urfave/cli.v1"

	"github.com/vmware/govmomi/list"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/cmd/vic-machine/create"
	"github.com/vmware/vic/lib/apiservers/service/models"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/decode"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/pkg/trace"
)

type mockFinder struct {
	path string
}

func (mf mockFinder) Element(ctx context.Context, t types.ManagedObjectReference) (*list.Element, error) {
	return &list.Element{
		Path: t.Value,
	}, nil
}

func TestFromManagedObject(t *testing.T) {
	op := trace.NewOperation(context.Background(), "TestFromManagedObject")
	var m *models.ManagedObject

	expected := ""
	actual, err := decode.FromManagedObject(op, nil, "t", m)
	assert.NoError(t, err, "Expected nil error, got %#v", err)
	assert.Equal(t, expected, actual)

	m = &models.ManagedObject{
		Name: "testManagedObject",
	}

	mf := mockFinder{}

	expected = m.Name
	actual, err = decode.FromManagedObject(op, mf, "t", m)
	assert.NoError(t, err, "Expected nil error, got %#v", err)
	assert.Equal(t, expected, actual)

	m.ID = "testID"

	expected = m.ID
	actual, err = decode.FromManagedObject(op, mf, "t", m)
	assert.NoError(t, err, "Expected nil error, got %#v", err)
	assert.Equal(t, expected, actual)

	m.Name = ""

	expected = m.ID
	actual, err = decode.FromManagedObject(op, mf, "t", m)
	assert.NoError(t, err, "Expected nil error, got %#v", err)
	assert.Equal(t, expected, actual)
}

func TestFromCIDR(t *testing.T) {
	var m models.CIDR

	expected := ""
	actual := decode.FromCIDR(&m)
	assert.Equal(t, expected, actual)

	m = "10.10.1.0/32"

	expected = string(m)
	actual = decode.FromCIDR(&m)
	assert.Equal(t, expected, actual)
}

func TestFromGateway(t *testing.T) {
	var m *models.Gateway

	expected := ""
	actual := decode.FromGateway(m)
	assert.Equal(t, expected, actual)

	m = &models.Gateway{
		Address: "192.168.31.37",
		RoutingDestinations: []models.CIDR{
			"192.168.1.1/24",
			"172.17.0.1/24",
		},
	}

	expected = "192.168.1.1/24,172.17.0.1/24:192.168.31.37"
	actual = decode.FromGateway(m)
	assert.Equal(t, expected, actual)
}

func TestCreateVCH(t *testing.T) {
	vch := &models.VCH{
		Name:  "test-vch",
		Debug: 3,
		Compute: &models.VCHCompute{
			Resource: &models.ManagedObject{
				Name: "TestCluster",
			},
		},
		Storage: &models.VCHStorage{
			ImageStores: []string{
				"ds://test/datastore",
			},
		},
		Network: &models.VCHNetwork{
			Bridge: &models.VCHNetworkBridge{
				IPRange: "17.16.0.0/12",
				PortGroup: &models.ManagedObject{
					ID:   "bridge", // required for mocked finder to work
					Name: "bridge",
				},
			},
			Public: &models.Network{
				PortGroup: &models.ManagedObject{
					ID:   "public", // required for mock finder to work
					Name: "public",
				},
			},
		},
		Registry: &models.VCHRegistry{
			ImageFetchProxy: &models.VCHRegistryImageFetchProxy{
				HTTP:  "http://example.com",
				HTTPS: "https://example.com",
			},
			Insecure: []string{
				"https://insecure.example.com",
			},
			Whitelist: []string{
				"10.0.0.0/8",
			},
		},
		Auth: &models.VCHAuth{
			Server: &models.VCHAuthServer{
				Generate: &models.VCHAuthServerGenerate{
					Cname: "vch.example.com",
					Organization: []string{
						"VMware, Inc.",
					},
					Size: &models.ValueBits{
						Value: models.Value{Value: 2048},
						Units: "bits",
					},
				},
			},
		},
		SyslogAddr: "tcp://syslog.example.com:4444",
		Container: &models.VCHContainer{
			NameConvention: "container-{id}",
		},
	}

	op := trace.NewOperation(context.Background(), "testing")
	defer func() {
		err := os.RemoveAll("test-vch")
		assert.NoError(t, err, "Error removing temp directory: %s", err)
	}()

	pass := "testpass"
	data := &data.Data{
		Target: &common.Target{
			URL:      &url.URL{Host: "10.10.1.2"},
			User:     "testuser",
			Password: &pass,
		},
	}

	ca := newCreate()
	ca.Data = data
	ca.DisplayName = "test-vch"
	err := ca.ProcessParams(op)
	assert.NoError(t, err, "Error while processing params: %s", err)
	op.Infof("ca EnvFile: %s", ca.Certs.EnvFile)

	mf := mockFinder{}

	cb, err := new(vchCreate).buildCreate(op, data, mf, vch)
	assert.NoError(t, err, "Error while processing params: %s", err)

	a := reflect.ValueOf(ca).Elem()
	b := reflect.ValueOf(cb).Elem()

	if err = compare(a, b, 0); err != nil {
		t.Fatalf("Error comparing create structs: %s", err)
	}
}

func newCreate() *create.Create {
	debug := 3
	ca := create.NewCreate()
	ca.Debug = common.Debug{Debug: &debug}
	ca.Compute = common.Compute{DisplayName: "TestCluster"}
	ca.ImageDatastorePath = "ds://test/datastore"
	ca.BridgeIPRange = "17.16.0.0/12"
	ca.BridgeNetworkName = "bridge"
	ca.PublicNetworkName = "public"
	ca.Certs.Cname = "vch.example.com"
	ca.Certs.Org = cli.StringSlice{"VMware, Inc."}
	ca.Certs.KeySize = 2048
	httpProxy := "http://example.com"
	httpsProxy := "https://example.com"
	ca.Proxies = common.Proxies{
		HTTPProxy:  &httpProxy,
		HTTPSProxy: &httpsProxy,
	}
	ca.Registries = common.Registries{
		InsecureRegistriesArg:  cli.StringSlice{"https://insecure.example.com"},
		WhitelistRegistriesArg: cli.StringSlice{"10.0.0.0/8"},
	}
	ca.SyslogAddr = "tcp://syslog.example.com:4444"
	ca.ContainerNameConvention = "container-{id}"
	ca.Certs.CertPath = "test-vch"
	ca.Certs.NoSaveToDisk = true

	return ca
}

func compare(a, b reflect.Value, index int) (err error) {
	switch a.Kind() {
	case reflect.Invalid, reflect.Uint8: // skip uint8 as generated cert data is not expected to match
		// NOP
	case reflect.Ptr:
		ae := a.Elem()
		be := b.Elem()
		if !ae.IsValid() != !be.IsValid() {
			return fmt.Errorf("Expected pointer validity to match for for %s", a.Type().Name())
		}
		return compare(ae, be, index)
	case reflect.Interface:
		return compare(a.Elem(), b.Elem(), index)
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if err = compare(a.Field(i), b.Field(i), i); err != nil {
				fmt.Printf("Field name a: %s, b: %s, index: %d\n", a.Type().Field(i).Name, b.Type().Field(i).Name, i)
				return err
			}
		}
	case reflect.Slice:
		m := min(a.Len(), b.Len())
		for i := 0; i < m; i++ {
			if err = compare(a.Index(i), b.Index(i), i); err != nil {
				return err
			}
		}
	case reflect.Map:
		keys := []string{}
		for _, key := range a.MapKeys() {
			keys = append(keys, key.String())
		}
		sort.Strings(keys)
		for i, key := range keys {
			if err = compare(a.MapIndex(reflect.ValueOf(key)), b.MapIndex(reflect.ValueOf(key)), i); err != nil {
				return err
			}
		}
	case reflect.String:
		if a.String() != b.String() {
			return fmt.Errorf("String fields not equal: %s != %s", a.String(), b.String())
		}
	default:
		if a.CanInterface() && b.CanInterface() {
			if a.Interface() != b.Interface() {
				return fmt.Errorf("Elements are not equal: %#v != %#v", a.Interface(), b.Interface())
			}
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
