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

package validate

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/simulator/esx"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/opsuser"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/rbac"
	"github.com/vmware/vic/pkg/vsphere/session"
)

func init() {
	// Globally enable the vSPC firewall rule by modifying the source template directly
	simulator.EnableRuleset(&esx.HostFirewallInfo, "vSPC") // TODO: can't use management.RulesetID here due to import cycle
}

func TestParseURL(t *testing.T) {
	var hosts = []string{
		"host.domain.com",
		"host.domain.com:123",
		"1.2.3.4",
		"1.2.3.4:10",
		"[2001:4860:0:2001::68]",
		"[2001:db8:1f70::999:de8:7648:6e8]:123",
	}

	for _, urlString := range hosts {
		u, err := ParseURL(urlString)
		assert.Nil(t, err)
		assert.Equal(t, u.String(), "https://"+urlString)
		// Null the scheme
		u.Scheme = ""
		assert.Equal(t, u.String(), "//"+urlString)
		assert.Equal(t, u.Host, urlString)
	}

	// Add path to create a more significant URL
	var urls = []string{}

	for i, h := range hosts {
		url := fmt.Sprintf("%s/path%d/test", h, i)
		urls = append(urls, url)
	}

	for i, urlString := range urls {
		u, err := ParseURL(urlString)
		assert.Nil(t, err)
		assert.Equal(t, u.String(), "https://"+urlString)

		// Null the scheme
		u.Scheme = ""
		assert.Equal(t, u.String(), "//"+urlString)

		// Check host
		assert.Equal(t, u.Host, hosts[i])
		// Check path
		path := fmt.Sprintf("/path%d/test", i)
		assert.Equal(t, u.Path, path)
		// Check concatenation
		assert.Equal(t, u.Host+u.Path, urlString)
	}

	// Add an HTTP scheme to verify that it is preserved
	var urlsWithHTTPScheme = []string{}

	for _, u := range urls {
		uws := fmt.Sprintf("http://%s", u)
		urlsWithHTTPScheme = append(urlsWithHTTPScheme, uws)
	}

	for _, urlString := range urlsWithHTTPScheme {
		u, err := ParseURL(urlString)
		fmt.Printf("UrlString: %s\n", u.String())
		assert.Nil(t, err)
		assert.Equal(t, u.String(), urlString)
	}

	var invalidUrls = []string{
		"[2001:db8/path",
		"1.2.3.4\\path",
	}

	for _, urlString := range invalidUrls {
		_, err := ParseURL(urlString)
		assert.NotNil(t, err)
	}
}

func TestMain(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	trace.Logger.Level = log.DebugLevel
	ctx := context.Background()

	for i, model := range []*simulator.Model{simulator.ESX(), simulator.VPX()} {
		t.Logf("%d", i)
		model.Datastore = 3
		defer model.Remove()
		err := model.Create()
		if err != nil {
			t.Fatal(err)
		}

		s := model.Service.NewServer()
		defer s.Close()

		s.URL.User = url.UserPassword("user", "pass")
		s.URL.Path = ""
		t.Logf("server URL: %s", s.URL)

		var input *data.Data
		if i == 0 {
			input = getESXData(s.URL)
		} else {
			input = getVPXData(s.URL)
		}
		if err != nil {
			t.Fatal(err)
		}

		opsUser := "ops-user-name"
		opsPassword := "ops-user-password"
		input.OpsCredentials.OpsUser = &opsUser
		input.OpsCredentials.OpsPassword = &opsPassword

		validator, err := NewValidator(ctx, input)
		if err != nil {
			t.Errorf("Failed to new validator: %s", err)
		}
		ds, _ := validator.Session.Finder.Datastore(validator.Context, "LocalDS_1")
		simulator.Map.Get(ds.Reference()).(mo.Entity).Entity().Name = "Local DS_0"

		ds, _ = validator.Session.Finder.Datastore(validator.Context, "LocalDS_2")
		simulator.Map.Get(ds.Reference()).(mo.Entity).Entity().Name = `😗`

		t.Logf("session pool: %s", validator.Session.Pool)
		if err = createPool(ctx, validator.Session, input.ComputeResourcePath, "validator", t); err != nil {
			t.Errorf("Unable to create resource pool: %s", err)
		}

		conf := testCompute(validator, input, t)
		testTargets(validator, input, conf, t)
		testStorage(validator, input, conf, t)
	}
}

func TestGrantPerms(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	trace.Logger.Level = log.DebugLevel
	op := trace.FromContext(context.Background(), "TestGrantPerms")

	for i, model := range []*simulator.Model{simulator.ESX(), simulator.VPX()} {
		t.Logf("%d", i)
		model.Datastore = 3
		defer model.Remove()
		err := model.Create()
		if err != nil {
			t.Fatal(err)
		}

		s := model.Service.NewServer()
		defer s.Close()

		s.URL.User = url.UserPassword("user", "pass")
		s.URL.Path = ""
		t.Logf("server URL: %s", s.URL)

		var input *data.Data
		if i == 0 {
			input = getESXData(s.URL)
		} else {
			input = getVPXData(s.URL)
		}
		if err != nil {
			t.Fatal(err)
		}

		conf := &config.VirtualContainerHostConfigSpec{}
		opsUser := "ops-user-name"
		opsPassword := "ops-user-password"
		opsGrantPermissions := true
		input.OpsCredentials.OpsUser = &opsUser
		input.OpsCredentials.OpsPassword = &opsPassword

		// Test 1: current value in conf is the empty string,
		// input is true, after validation
		// the new GrantPerms value in conf should be
		// "config.AddPerms"
		conf.GrantPermsLevel = ""
		input.OpsCredentials.GrantPerms = &opsGrantPermissions
		validator, err := NewValidator(op, input)
		require.NoError(t, err, "Failed to create validator")

		validator.credentials(op, input, conf)
		assert.Equal(t, conf.GrantPermsLevel, config.AddPerms)

		// Test 2: current value in conf is the empty string,
		// input is nil, after validation
		// the new GrantPerms value in conf should be
		// the empty string
		conf.GrantPermsLevel = ""
		input.OpsCredentials.GrantPerms = nil
		validator, err = NewValidator(op, input)
		require.NoError(t, err, "Failed to create validator")

		validator.credentials(op, input, conf)
		assert.Equal(t, conf.GrantPermsLevel, "")

		// Test 3: current value in conf is the empty string,
		// input is false, after validation
		// the new GrantPerms value in conf should be
		// the empty string
		conf.GrantPermsLevel = ""
		opsGrantPermissions = false
		input.OpsCredentials.GrantPerms = &opsGrantPermissions
		validator, err = NewValidator(op, input)
		require.NoError(t, err, "Failed to create validator")

		validator.credentials(op, input, conf)
		assert.Equal(t, conf.GrantPermsLevel, "")

		// Test 4: current value in conf is "config.AddPerms",
		// input is true, after validation
		// the new GrantPerms value in conf should be
		// "config.AddPerms"
		conf.GrantPermsLevel = config.AddPerms
		opsGrantPermissions = true
		input.OpsCredentials.GrantPerms = &opsGrantPermissions
		validator, err = NewValidator(op, input)
		require.NoError(t, err, "Failed to create validator")

		validator.credentials(op, input, conf)
		assert.Equal(t, conf.GrantPermsLevel, config.AddPerms)

		// Test 5: current value in conf is "config.AddPerms",
		// input is nil, after validation
		// the new GrantPerms value in conf should be
		// "config.AddPerms"
		conf.GrantPermsLevel = config.AddPerms
		input.OpsCredentials.GrantPerms = nil
		validator, err = NewValidator(op, input)
		require.NoError(t, err, "Failed to create validator")

		validator.credentials(op, input, conf)
		assert.Equal(t, conf.GrantPermsLevel, config.AddPerms)

		// Test 6: current value in conf is "config.AddPerms",
		// input is false, after validation
		// the new GrantPerms value in conf should be
		// the empty string
		conf.GrantPermsLevel = config.AddPerms
		opsGrantPermissions = false
		input.OpsCredentials.GrantPerms = &opsGrantPermissions
		validator, err = NewValidator(op, input)
		require.NoError(t, err, "Failed to create validator")

		validator.credentials(op, input, conf)
		assert.Equal(t, conf.GrantPermsLevel, "")
	}
}

func getESXData(testURL *url.URL) *data.Data {
	result := data.NewData()
	testURL.Path = testURL.Path + "/ha-datacenter"
	user := testURL.User.Username()
	result.OpsCredentials.OpsUser = &user
	passwd, _ := testURL.User.Password()
	result.OpsCredentials.OpsPassword = &passwd
	result.URL = testURL
	result.DisplayName = "test001"
	result.ComputeResourcePath = "/ha-datacenter/host/localhost.localdomain/Resources"
	result.ImageDatastorePath = "LocalDS_0"
	result.BridgeNetworkName = "bridge"
	_, result.BridgeIPRange, _ = net.ParseCIDR("172.16.0.0/12")
	result.ManagementNetwork.Name = "VM Network"
	result.PublicNetwork.Name = "VM Network"
	result.VolumeLocations = make(map[string]*url.URL)
	testVolumeStoreURL := &url.URL{
		Host: "LocalDS_0",
		Path: "volumes/test",
	}
	result.VolumeLocations["volume-store"] = testVolumeStoreURL
	return result
}

func getVPXData(testURL *url.URL) *data.Data {
	result := data.NewData()
	testURL.Path = testURL.Path + "/DC0"
	user := testURL.User.Username()
	result.OpsCredentials.OpsUser = &user
	passwd, _ := testURL.User.Password()
	result.OpsCredentials.OpsPassword = &passwd
	result.URL = testURL
	result.DisplayName = "test001"
	result.ComputeResourcePath = "/DC0/host/DC0_C0/Resources"
	result.ImageDatastorePath = "LocalDS_0"
	result.PublicNetwork.Name = "VM Network"
	result.BridgeNetworkName = "bridge"
	_, result.BridgeIPRange, _ = net.ParseCIDR("172.16.0.0/12")

	result.VolumeLocations = make(map[string]*url.URL)
	testVolumeStoreURL := &url.URL{
		Host: "LocalDS_0",
		Path: "volumes/test",
	}
	result.VolumeLocations["volume-store"] = testVolumeStoreURL
	return result
}

func createPool(ctx context.Context, sess *session.Session, poolPath string, name string, t *testing.T) error {
	rp, err := sess.Finder.ResourcePool(ctx, poolPath)
	if err != nil {
		t.Logf("Failed to get parent pool: %s", err)
		return err
	}
	t.Logf("Creating Resource Pool %s", name)
	resSpec := types.DefaultResourceConfigSpec()
	_, err = rp.Create(ctx, name, resSpec)
	if err != nil {
		t.Logf("Failed to create resource pool %s: %s", name, err)
		return err
	}
	return nil
}

func testCompute(v *Validator, input *data.Data, t *testing.T) *config.VirtualContainerHostConfigSpec {
	op := trace.FromContext(v.Context, "testCompute")

	tests := []struct {
		path   string
		vc     bool
		hasErr bool
	}{
		{"DC0_C0/Resources/validator", true, false},
		{"DC0_C0/validator", true, true},
		{"validator", true, false},
		{"DC0_C0/test", true, true},
		{"/DC0_C1/test", true, true},
		{"/DC0_C1/test", true, true},
		{"/DC0/host/DC0_C1/Resources/validator", true, true},
		{"/DC1/host/DC0_C1/Resources/validator", true, true},
		{"DC0_H0/Resources", true, false},
		{"DC0_H0", true, false},
		{"/DC0/host/DC0_C0/Resources/validator", true, false},
		{"localhost.localdomain/Resources/validator", false, false},
		{"validator", false, false},
		{"test", false, true},
		{"/ha-datacenter/host/localhost.localdomain/Resources/validator", false, false},
	}
	conf := &config.VirtualContainerHostConfigSpec{}

	for _, test := range tests {
		if v.isVC && !test.vc {
			continue
		}
		if !v.isVC && test.vc {
			continue
		}
		t.Logf("%+v", test)
		input.ComputeResourcePath = test.path
		v.compute(op, input, conf)
		v.ListIssues(op)
		if !test.hasErr {
			assert.Equal(t, 0, len(v.issues))
		} else {
			assert.True(t, len(v.issues) > 0, "Should have errors")
		}
		v.issues = nil
	}
	return conf
}

func testTargets(v *Validator, input *data.Data, conf *config.VirtualContainerHostConfigSpec, t *testing.T) {
	op := trace.FromContext(v.Context, "testTargets")

	v.target(op, input, conf)
	v.credentials(op, input, conf)

	u, err := url.Parse(conf.Target)
	assert.NoError(t, err)
	assert.Nil(t, u.User)
	assert.NotEmpty(t, conf.Token)
	assert.NotEmpty(t, conf.Username)

}

func testStorage(v *Validator, input *data.Data, conf *config.VirtualContainerHostConfigSpec, t *testing.T) {
	op := trace.FromContext(v.Context, "testStorage")

	// specifically ignoring err here because we do not care about the parse result.
	testURL1, _ := url.Parse("LocalDS_0/volumes/volume1")
	testURL2, _ := url.Parse("LocalDS_0/volumes/volume2")
	testURL2.Scheme = "ds"
	testURL3, _ := url.Parse("LocalDS_0/volumes/volume1")
	testURL3.Scheme = "ds"

	// These two should report errors due to bad characters in the url. These should test how DatastoreHelper handles a nil or malformed url.
	testURL4, _ := url.Parse("😗/volumes/volume1")
	testURL5, _ := url.Parse("ds://😗/volumes/volume2")

	testURL6, _ := url.Parse("LocalDS_1/volumes/volume1")
	testURL7, _ := url.Parse("ds://LocalDS_1/volumes/volume2")
	testURL8, _ := url.Parse("")
	testURL9, _ := url.Parse("ds://")

	// positive nfs case
	nfsTestURL1, _ := url.Parse("nfs://prod.shared.storage/vchprod/volumes")

	// the two current negative nfs cases for validation
	nfsTestURL2, _ := url.Parse("nfs:///no/host/here")
	nfsTestURL3, _ := url.Parse("nfs://no.actual.path")

	tests := []struct {
		image         string
		volumes       map[string]*url.URL
		hasErr        bool
		expectImage   string
		expectVolumes map[string]*url.URL
	}{
		{"LocalDS_0",
			map[string]*url.URL{"volume1": testURL1,
				"volume2": testURL2},
			false,
			"ds://LocalDS_0",
			map[string]*url.URL{"volume1": testURL3,
				"volume2": testURL2}},
		{"LocalDS_0/images",
			map[string]*url.URL{"volume1": testURL1,
				"volume2": testURL2},
			false,
			"ds://LocalDS_0/images",
			map[string]*url.URL{"volume1": testURL3,
				"volume2": testURL2}},

		{"ds://LocalDS_0/images",
			map[string]*url.URL{"volume1": testURL1,
				"volume2": testURL2},
			false,
			"ds://LocalDS_0/images",
			map[string]*url.URL{"volume1": testURL3,
				"volume2": testURL2}},

		{"ds://LocalDS_0/images/xyz",
			map[string]*url.URL{"volume1": testURL1,
				"volume2": testURL2},
			false,
			"ds://LocalDS_0/images/xyz",
			map[string]*url.URL{"volume1": testURL3,
				"volume2": testURL2}},

		{"ds://😗",
			map[string]*url.URL{"volume1": testURL4,
				"volume2": testURL5},
			true,
			"ds://😗/test001",
			nil},

		{"ds://LocalDS_0",
			map[string]*url.URL{"volume1": testURL6,
				"volume2": testURL7},
			true,
			"ds://LocalDS_0",
			nil},

		{"LocalDS_0",
			map[string]*url.URL{"volume1": testURL6,
				"volume2": testURL7},
			true,
			"ds://LocalDS_0",
			nil},

		{"LocalDS_0",
			map[string]*url.URL{"volume1": testURL6,
				"volume2": testURL7},
			true,
			"ds://LocalDS_0",
			nil},

		{"",
			map[string]*url.URL{"volume1": testURL8,
				"volume2": testURL9},
			true,
			"",
			nil},

		{"ds://",
			map[string]*url.URL{"volume1": testURL8,
				"volume2": testURL9},
			true,
			"",
			nil},
		// below here lies the setup for nfs validation checks

		{"LocalDS_0",
			map[string]*url.URL{"volume1": nfsTestURL1},
			false,
			"ds://LocalDS_0",
			map[string]*url.URL{"volume1": nfsTestURL1}},

		{"LocalDS_0",
			map[string]*url.URL{"volume1": nfsTestURL1,
				"volume2": nfsTestURL2},
			true,
			"ds://LocalDS_0",
			map[string]*url.URL{"volume1": nfsTestURL1}},
		{"LocalDS_0",
			map[string]*url.URL{"volume1": nfsTestURL1,
				"volume2": nfsTestURL3},
			true,
			"ds://LocalDS_0",
			map[string]*url.URL{"volume1": nfsTestURL1}},
		{"LocalDS_0",
			map[string]*url.URL{"volume1": nfsTestURL3,
				"volume2": nfsTestURL2},
			true,
			"ds://LocalDS_0",
			nil},
		// below here lies the mixed store validation checks
		{"LocalDS_0",
			map[string]*url.URL{"volume1": testURL1,
				"volume2": nfsTestURL1,
				"volume3": nfsTestURL2,
				"volume4": testURL4,
			},
			true,
			"ds://LocalDS_0",
			map[string]*url.URL{"volume1": testURL3,
				"volume2": nfsTestURL1}},
	}

	for _, test := range tests {
		t.Logf("%+v", test)
		input.ImageDatastorePath = test.image
		input.VolumeLocations = test.volumes
		v.storage(op, input, conf)
		v.ListIssues(op)
		if !test.hasErr {
			assert.Equal(t, 0, len(v.issues))
			assert.Equal(t, test.expectImage, conf.ImageStores[0].String())
			conf.ImageStores = conf.ImageStores[1:]
			for key, volume := range conf.VolumeLocations {
				if _, ok := test.expectVolumes[key]; !ok {
					assert.Fail(t, "Could not find volume store that was expected to present", "volume : %s", volume.String())
				} else {
					assert.Equal(t, test.expectVolumes[key].String(), volume.String())
				}
			}
		} else {
			assert.True(t, len(v.issues) > 0, "Should have errors")
		}
		v.issues = nil
		conf.VolumeLocations = nil
	}
}

// TODO: vcsim should support UpdateOptions
type testOptionManager struct {
	*simulator.OptionManager
}

func (m *testOptionManager) UpdateOptions(req *types.UpdateOptions) soap.HasFault {
	m.Setting = append(req.ChangedValue, m.Setting...)

	return &methods.UpdateOptionsBody{
		Res: new(types.UpdateOptionsResponse),
	}
}

func TestValidateWithFolders(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	op := trace.NewOperation(context.Background(), "TestValidateWithFolders")

	m := simulator.VPX()
	m.Datacenter = 3
	m.Folder = 2
	m.Datastore = 2
	m.ClusterHost = 3
	m.Pool = 1

	defer m.Remove()

	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	m.Service.TLS = new(tls.Config)
	s := m.Service.NewServer()
	defer s.Close()

	om := simulator.Map.Get(*m.ServiceContent.Setting).(*simulator.OptionManager)
	simulator.Map.Put(&testOptionManager{om})

	input := data.NewData()
	input.URL = &url.URL{
		Scheme: s.URL.Scheme,
		Host:   s.URL.Host,
	}

	newShouldFail := true
	license := simulator.EvalLicense
	simulator.EvalLicense.Properties = nil // erase features

	var validator *Validator
	var dc string

	// Cover various failure paths while we're at it
	steps := []func(){
		func() {},
		func() {
			input.Thumbprint = "nope"
		},
		func() {
			input.Force = true
			input.Thumbprint = ""
		},
		func() {
			input.Force = false
			input.Thumbprint = s.CertificateInfo().ThumbprintSHA1
		},
		func() {
			input.URL.Path = "/"
			input.URL.User = s.URL.User
			newShouldFail = false
			if _, err = validator.ValidateCompute(op, input, false); err != nil {
				t.Error(err)
			}
		},
		func() {
			input.URL.Path = "/enoent" // Datacenter "enoent" in --target is not found
		},
		func() {
			input.URL.Path = "/DC1/sorry" // --target should only specify datacenter in the path
		},
		func() {
			input.URL.Path = "/DC1" // ok
			dc = input.URL.Path
		},
		func() {
			if _, err = validator.ValidateCompute(op, input, true); err == nil {
				t.Error("expected error")
			}
			input.ComputeResourcePath = "enoent"
		},
		func() {
			input.ComputeResourcePath = "DC1_C0"
		},
		func() {
			input.PublicNetwork.Name = "enoent"
		},
		func() {
			input.PublicNetwork.Name = "VM Network"
			input.ManagementNetwork.Name = input.PublicNetwork.Name
			input.ClientNetwork.Name = input.PublicNetwork.Name
			input.BridgeNetworkName = "DC1_DVPG0"
		},
		func() {
			input.ScratchSize = "10GB"
			p, _ := s.URL.User.Password()
			input.OpsCredentials.OpsPassword = &p
		},
		func() {
			input.ImageDatastorePath = "enoent"
		},
		func() {
			input.ImageDatastorePath = "LocalDS_*" // > 1
		},
		func() {
			input.ImageDatastorePath = "LocalDS_0"
		},
		func() {
			// TODO: volume
		},
		func() {
			user := s.URL.User.Username()
			input.OpsCredentials.OpsUser = &user
		},
		func() {
			simulator.EvalLicense.Properties = license.Properties // restore license features
		},
	}

	for i, step := range steps {
		if testing.Verbose() {
			fmt.Fprintf(os.Stderr, "TestValidateVPX(%d)%s\n", i, strings.Repeat(".", 30))
		}
		step()

		validator, err = NewValidator(op, input)
		if err != nil {
			continue
		}

		if newShouldFail {
			t.Fatalf("%d: expected error", i)
		}

		_, err = validator.Validate(op, input)
		if i == len(steps)-1 {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			if err == nil {
				t.Fatal("expected error")
			}
		}

		if dc != "" {
			// NewValidator has the side-effect of setting input.URL.Path=""
			input.URL.Path = dc
		}
	}

	// we have valid input at this point, test various compute-resource suggestions
	vs := validator.Session
	crs := []struct {
		flag    string
		pool    string
		cluster string
	}{
		{"*", "", ""},         // MultipleFoundError
		{"Resources", "", ""}, // MultipleFoundError
		{"DC1_[CH]0", "", ""}, // MultipleFoundError
		{"DC1_C0_RP1", "/F0/DC1/host/F0/DC1_C0/Resources/DC1_C0_RP1", "/F0/DC1/host/F0/DC1_C0"}, // ResourcePool (nested)
		{"DC1_H0", "/F0/DC1/host/F0/DC1_H0/Resources", "/F0/DC1/host/F0/DC1_H0"},                // Host (standalone)
		{"DC1_C0", "/F0/DC1/host/F0/DC1_C0/Resources", "/F0/DC1/host/F0/DC1_C0"},                // Cluster
	}

	for _, cr := range crs {
		vs.Pool = nil
		vs.PoolPath = ""

		vs.Cluster = nil
		vs.ClusterPath = ""

		_, err = validator.ResourcePoolHelper(op, cr.flag)

		if vs.ClusterPath != cr.cluster {
			t.Errorf("%s ClusterPath=%s", cr.flag, vs.ClusterPath)
		}

		if vs.PoolPath != cr.pool {
			t.Errorf("%s PoolPath=%s", cr.flag, vs.PoolPath)
		}

		if err == nil {
			continue
		}

		switch err.(type) {
		case *find.MultipleFoundError:
			// expected
		default:
			t.Errorf("ResourcePoolHelper(%s): %s", cr.flag, err)
		}
	}

	// cover some other paths now that we have a valid config
	spec, err := validator.ValidateTarget(op, input)
	if err != nil {
		t.Fatal(err)
	}

	validator.AddDeprecatedFields(op, spec, input)

	_, err = CreateFromVCHConfig(op, spec, vs)
	if err != nil {
		t.Fatal(err)
	}

	// force vim25.NewClient to fail
	simulator.Map.Remove(methods.ServiceInstance)
	validator.credentials(op, input, spec)

	// cover some of the return+error paths for certs (TODO: move this elsewhere and include valid data)
	validator.certificate(op, input, spec)
	validator.certificateAuthorities(op, input, spec)

	input.CertPEM = s.Certificate().Raw
	validator.certificate(op, input, spec)
	validator.certificateAuthorities(op, input, spec)

	input.ClientCAs = []byte{1}
	validator.certificateAuthorities(op, input, spec)

	validator.registries(op, input, spec)
	input.RegistryCAs = input.ClientCAs
	validator.registries(op, input, spec)
}

func TestValidateWithESX(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	op := trace.NewOperation(context.Background(), "TestValidateWithFolders")

	m := simulator.ESX()
	defer m.Remove()

	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	m.Service.TLS = new(tls.Config)
	s := m.Service.NewServer()
	defer s.Close()

	input := data.NewData()
	input.URL = &url.URL{
		Path: s.URL.Host,
		User: s.URL.User,
	}

	input.Thumbprint = s.CertificateInfo().ThumbprintSHA1

	steps := []func(){
		func() {
			input.ComputeResourcePath = "enoent"
		},
		func() {
			input.PublicNetwork.Name = "enoent"
		},
		func() {
			input.ImageDatastorePath = "enoent"
		},
		func() {
			input.ImageDatastorePath = "enoent"
		},
		func() {
			input = getESXData(s.URL)
			input.URL.Path = "/"
			input.ScratchSize = "10GB"
			input.Force = true
		},
	}

	var validator *Validator

	for i, step := range steps {
		if testing.Verbose() {
			fmt.Fprintf(os.Stderr, "TestValidateESX(%d)%s\n", i, strings.Repeat(".", 30))
		}

		step()

		validator, err = NewValidator(op, input)
		if err != nil {
			t.Fatal(err)
		}

		validator.AllowEmptyDC()

		_, err = validator.Validate(op, input)
		if i == len(steps)-1 {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			if err == nil {
				t.Fatal("expected error")
			}
		}
	}

	// cover some errors paths by destroying this ESX system
	ref := esx.HostSystem.Reference()
	host := simulator.Map.Get(ref).(*simulator.HostSystem)

	steps = []func(){
		func() {
			host.Summary.ManagementServerIp = "owned"
		},
		func() {
			host.Summary.ManagementServerIp = ""
			simulator.Map.Remove(ref) // remove the host, forcing Finder.DefaultHostSystem to fail
		},
	}

	for i, step := range steps {
		step()

		validator.managedbyVC(op)
		issues := validator.GetIssues()

		if len(issues) != 1 {
			t.Errorf("%d issues: %s", i, issues)
		}
		validator.ClearIssues()
	}

	simulator.Map.Remove(esx.Datacenter.Reference()) // goodnight now.
	validator.suggestDatacenter(op)
}

func TestDCReadOnlyPermsFromConfigSimulatorVPX(t *testing.T) {
	ctx := context.Background()
	m := simulator.VPX()

	m.Datacenter = 3
	m.Folder = 2
	m.Pool = 1
	m.App = 1
	m.Pod = 1

	defer m.Remove()

	err := m.Create()
	require.NoError(t, err)

	s := m.Service.NewServer()
	defer s.Close()

	fmt.Println(s.URL.String())

	input := GetVcsimInputConfig(ctx, s.URL)
	require.NotNil(t, input)
	v, err := NewValidator(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, v)
	configSpec, err := v.VcsimValidate(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, configSpec)

	// Set up the Authz Manager
	mgr := opsuser.NewRBACManager(ctx, v.Session.Vim25(), v.Session, &opsuser.DCReadOnlyConf, configSpec)

	resourcePermission, err := mgr.SetupDCReadOnlyPermissions(ctx)
	require.NoError(t, err)
	require.NotNil(t, resourcePermission)
	resourcePermissions := []rbac.ResourcePermission{*resourcePermission}
	require.True(t, len(mgr.AuthzManager.Config.Resources) >= len(resourcePermissions))

	rbac.VerifyResourcePermissions(ctx, t, mgr.AuthzManager, resourcePermissions)
}

func TestOpsUserPermsFromConfigSimulatorVPX(t *testing.T) {
	ctx := context.Background()
	m := simulator.VPX()

	m.Datacenter = 3
	m.Folder = 2
	m.Pool = 1
	m.App = 1
	m.Pod = 1

	defer m.Remove()

	err := m.Create()
	require.NoError(t, err)

	s := m.Service.NewServer()
	defer s.Close()

	fmt.Println(s.URL.String())

	config := &session.Config{
		Service:   s.URL.String(),
		Insecure:  true,
		Keepalive: time.Duration(5) * time.Minute,
	}
	sess, err := session.NewSession(config).Connect(ctx)
	require.NoError(t, err)

	input := GetVcsimInputConfig(ctx, s.URL)
	require.NotNil(t, input)
	v, err := NewValidator(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, v)
	configSpec, err := v.VcsimValidate(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, configSpec)

	// Set up the Authz Manager
	mgr := opsuser.NewRBACManager(ctx, sess.Vim25(), nil, &opsuser.OpsuserRBACConf, configSpec)

	resourcePermissions, err := mgr.SetupRolesAndPermissions(ctx)
	require.NoError(t, err)
	am := mgr.AuthzManager
	defer rbac.Cleanup(ctx, t, am, true)
	require.True(t, len(am.Config.Resources) >= len(resourcePermissions))

	rbac.VerifyResourcePermissions(ctx, t, mgr.AuthzManager, resourcePermissions)
}
