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

package vicadmin

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/opts"

	"github.com/vishvananda/netlink"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/guest"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/lib/tether/shared"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

type Validator struct {
	Hostname         string
	Version          string
	FirewallStatus   template.HTML
	FirewallIssues   template.HTML
	LicenseStatus    template.HTML
	LicenseIssues    template.HTML
	NetworkStatus    template.HTML
	NetworkIssues    template.HTML
	StorageRemaining template.HTML
	HostIP           string
	DockerPort       string
	VCHStatus        template.HTML
	VCHIssues        template.HTML
	VCHReachable     bool
	SystemTime       string
}

const (
	GoodStatus     = template.HTML(`<i class="icon-ok"></i>`)
	BadStatus      = template.HTML(`<i class="icon-attention"></i>`)
	DefaultVCHName = ` `
)

func GetMgmtIP() net.IPNet {
	var mgmtIP net.IPNet
	// management alias may not be present, try others if not found
	link := LinkByOneOfNameOrAlias(config.ManagementNetworkName, "public", "client")
	if link == nil {
		log.Error("unable to find any interfaces when searching for mgmt IP")
		return mgmtIP
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		log.Errorf("error getting address list: %s", err)
		return mgmtIP
	}
	if len(addrs) == 0 {
		log.Warnf("no addresses on interface when searching for mgmt IP")
		return mgmtIP
	}
	if len(addrs) > 1 {
		log.Warnf("multiple addresses on interface when searching for mgmt IP, using first")
	}
	return *addrs[0].IPNet
}

func LinkByOneOfNameOrAlias(name ...string) netlink.Link {
	for _, n := range name {
		// #nosec: Errors unhandled.
		if l, _ := netlink.LinkByName(n); l != nil {
			return l
		}
		// #nosec: Errors unhandled.
		if l, _ := netlink.LinkByAlias(n); l != nil {
			return l
		}
	}
	return nil
}

func NewValidator(ctx context.Context, vch *config.VirtualContainerHostConfigSpec, sess *session.Session) *Validator {
	defer trace.End(trace.Begin(""))
	log.Infof("Creating new validator")
	v := &Validator{}

	if version.Version != "" {
		v.Version = version.Version
	}
	log.Infof("Setting version to %s", v.Version)

	if err := v.GetVCHName(ctx, sess); err != nil {
		log.Errorf("Failed to obtain the VCH name: %s", err)
	}

	// System time
	v.SystemTime = time.Now().Format(time.UnixDate)

	if sess == nil {
		// We can't connect to vSphere
		v.VCHReachable = false
		v.FirewallStatus = BadStatus
		v.FirewallIssues = template.HTML("")
		v.LicenseStatus = BadStatus
		v.LicenseIssues = template.HTML("")
	} else {
		v.VCHReachable = true
		// Firewall status check
		// #nosec: Errors unhandled.
		v2, _ := validate.CreateFromSession(ctx, sess)
		mgmtIP := GetMgmtIP()
		log.Infof("Using management IP %s for firewall check", mgmtIP)
		fwStatus := v2.CheckFirewallForTether(ctx, mgmtIP)
		v2.FirewallCheckOutput(ctx, fwStatus)

		firewallIssues := v2.GetIssues()

		if len(firewallIssues) == 0 {
			v.FirewallStatus = GoodStatus
			v.FirewallIssues = template.HTML("")
		} else {
			v.FirewallStatus = BadStatus
			for _, err := range firewallIssues {
				// #nosec: this method will not auto-escape HTML. Verify data is well formed.
				v.FirewallIssues = template.HTML(fmt.Sprintf("%s<span class=\"error-message\">%s</span>\n", v.FirewallIssues, err))
			}
		}

		// License status check
		v2.ClearIssues()
		v2.CheckLicense(ctx)
		licenseIssues := v2.GetIssues()

		if len(licenseIssues) == 0 {
			v.LicenseStatus = GoodStatus
			v.LicenseIssues = template.HTML("")
		} else {
			v.LicenseStatus = BadStatus
			for _, err := range licenseIssues {
				// #nosec: this method will not auto-escape HTML. Verify data is well formed.
				v.LicenseIssues = template.HTML(fmt.Sprintf("%s<span class=\"error-message\">%s</span>\n", v.LicenseIssues, err))
			}
		}
	}

	log.Infof("FirewallStatus set to: %s", v.FirewallStatus)
	log.Infof("FirewallIssues set to: %s", v.FirewallIssues)
	log.Infof("LicenseStatus set to: %s", v.LicenseStatus)
	log.Infof("LicenseIssues set to: %s", v.LicenseIssues)

	// Network Connection Check
	hosts := []string{
		"http://google.com",
		"https://docker.io",
	}

	nwErrors := []error{}

	// create a http client with a custom transport using the proxy from env vars
	client := &http.Client{Timeout: 10 * time.Second}
	// priority given to https proxies
	proxy := os.Getenv("VICADMIN_HTTPS_PROXY")
	if proxy == "" {
		proxy = os.Getenv("VICADMIN_HTTP_PROXY")
	}
	if proxy != "" {
		url, err := url.Parse(proxy)
		if err != nil {
			nwErrors = append(nwErrors, err)
		} else {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(url)}
		}
	}

	// perform the wan check
	var wg sync.WaitGroup
	wg.Add(len(hosts))
	errs := make(chan error, len(hosts))
	for _, host := range hosts {
		go func(host string) {
			defer wg.Done()
			log.Infof("Getting %s", host)
			_, err := client.Get(host)
			if err != nil {
				errs <- err
			}
		}(host)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		nwErrors = append(nwErrors, err)
	}

	if len(nwErrors) > 0 {
		v.NetworkStatus = BadStatus
		for _, err := range nwErrors {
			// #nosec: this method will not auto-escape HTML. Verify data is well formed.
			v.NetworkIssues = template.HTML(fmt.Sprintf("%s<span class=\"error-message\">%s</span>\n", v.NetworkIssues, err))
		}
	} else {
		v.NetworkStatus = GoodStatus
		v.NetworkIssues = template.HTML("")
	}
	log.Infof("NetworkStatus set to: %s", v.NetworkStatus)
	log.Infof("NetworkIssues set to: %s", v.NetworkIssues)

	// Retrieve Host IP Information and Set Docker Endpoint
	v.HostIP = vch.ExecutorConfig.Networks["client"].Assigned.IP.String()

	if vch.HostCertificate.IsNil() {
		v.DockerPort = fmt.Sprintf("%d", opts.DefaultHTTPPort)
	} else {
		v.DockerPort = fmt.Sprintf("%d", opts.DefaultTLSHTTPPort)
	}

	err := v.QueryDatastore(ctx, vch, sess)
	if err != nil {
		log.Errorf("Had a problem querying the datastores: %s", err.Error())
	}
	v.QueryVCHStatus(ctx, vch, sess)
	return v
}

type dsList []mo.Datastore

func (d dsList) Len() int           { return len(d) }
func (d dsList) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
func (d dsList) Less(i, j int) bool { return d[i].Name < d[j].Name }

// Obtain the VCH name from vsphere
func (v *Validator) GetVCHName(ctx context.Context, sess *session.Session) error {
	defer trace.End(trace.Begin(""))

	var err error

	if sess == nil {
		v.Hostname = DefaultVCHName
		return fmt.Errorf("session is nil")
	}

	self, err := guest.GetSelf(ctx, sess)
	if err != nil {
		v.Hostname = DefaultVCHName
		return fmt.Errorf("unknown self-reference: %s", err)
	}

	newVM := vm.NewVirtualMachineFromVM(ctx, sess, self)
	vchName, err := newVM.ObjectName(ctx)
	if err != nil {
		v.Hostname = DefaultVCHName
		return err
	}

	v.Hostname = vchName
	log.Infof("Setting the VCH name to %s", vchName)
	return nil
}

func (v *Validator) QueryDatastore(ctx context.Context, vch *config.VirtualContainerHostConfigSpec, sess *session.Session) error {
	if sess == nil {
		// If we can't connect to vSphere, don't display datastore info
		v.StorageRemaining = template.HTML("")
		return nil
	}

	var dataStores dsList
	dsNames := make(map[string]bool)

	for _, url := range vch.ImageStores {
		dsNames[url.Host] = true
	}

	for _, url := range vch.VolumeLocations {
		dsNames[url.Host] = true
	}

	for _, url := range vch.ContainerStores {
		dsNames[url.Host] = true
	}

	refs := []types.ManagedObjectReference{}
	for dsName := range dsNames {
		ds, err := sess.Finder.DatastoreOrDefault(ctx, dsName)
		if err != nil {
			log.Errorf("Unable to collect information for datastore %s: %s", dsName, err)
		} else {
			refs = append(refs, ds.Reference())
		}
	}

	if len(refs) == 0 {
		return fmt.Errorf("No datastore references found")
	}

	pc := property.DefaultCollector(sess.Client.Client)
	if pc == nil {
		return fmt.Errorf("Could not get default propery collector; prop-collector came back nil")
	}

	err := pc.Retrieve(ctx, refs, nil, &dataStores)
	if err != nil {
		log.Errorf("Error while accessing datastore: %s", err)
	}

	sort.Sort(dataStores)
	for _, ds := range dataStores {
		log.Infof("Datastore %s Status: %s", ds.Name, ds.OverallStatus)
		log.Infof("Datastore %s Free Space: %.1fGB", ds.Name, float64(ds.Summary.FreeSpace)/(1<<30))
		log.Infof("Datastore %s Capacity: %.1fGB", ds.Name, float64(ds.Summary.Capacity)/(1<<30))

		// #nosec: this method will not auto-escape HTML. Verify data is well formed.
		v.StorageRemaining = template.HTML(fmt.Sprintf(`%s
			<p class="card-text">
			  %s:
			  %.1f GB remaining
			</p>`, v.StorageRemaining, ds.Name, float64(ds.Summary.FreeSpace)/(1<<30)))
	}

	return nil
}

func (v *Validator) QueryVCHStatus(ctx context.Context, vch *config.VirtualContainerHostConfigSpec, sess *session.Session) {
	defer trace.End(trace.Begin(""))

	if sess == nil {
		// We can't connect to vSphere
		v.VCHStatus = BadStatus
		return
	}

	v.VCHIssues = template.HTML("")
	v.VCHStatus = GoodStatus

	procs := map[string]string{"vic-init": "vic-init"}

	// Extract required components from vchConfig
	// Only report on components with Restart set to true
	for service, sess := range vch.ExecutorConfig.Sessions {
		if !sess.Restart {
			continue
		}
		cmd := path.Base(sess.Cmd.Path)
		procs[service] = cmd
	}
	log.Infof("Processes to check: %+v", procs)

	for service, proc := range procs {
		log.Infof("Checking status of %s", proc)
		pid, err := ioutil.ReadFile(fmt.Sprintf("%s.pid", path.Join(shared.PIDFileDir(), proc)))
		if err != nil {
			// #nosec: this method will not auto-escape HTML. Verify data is well formed.
			v.VCHIssues = template.HTML(fmt.Sprintf("%s<span class=\"error-message\">%s service is not running</span>\n",
				v.VCHIssues, strings.Title(service)))
			log.Errorf("Process %s not running: %s", proc, err)
			continue
		}

		status, err := ioutil.ReadFile(path.Join("/proc", string(pid), "stat"))
		if err != nil {
			// #nosec: this method will not auto-escape HTML. Verify data is well formed.
			v.VCHIssues = template.HTML(fmt.Sprintf("%s<span class=\"error-message\">Unable to query service %s</span>\n",
				v.VCHIssues, strings.Title(service)))
			continue
		}

		fields := strings.Split(string(status), " ")
		// Field 3 is the current state as reported by the kernel
		switch fields[2][0] {
		// We're good
		case 'R', 'S', 'D':
			log.Infof("Process %s running as PID %s", proc, pid)
			break
		// Process has been killed, is dying, or a zombie
		case 'K', 'X', 'x', 'Z':
			// #nosec: this method will not auto-escape HTML. Verify data is well formed.
			v.VCHIssues = template.HTML(fmt.Sprintf("%s<span class=\"error-message\">%s has failed</span>\n",
				v.VCHIssues, strings.Title(service)))
		}
	}

	v.QueryVMGroupStatus(ctx, vch, sess)

	if v.VCHIssues != template.HTML("") {
		v.VCHStatus = BadStatus
	}
}

func (v *Validator) QueryVMGroupStatus(ctx context.Context, vch *config.VirtualContainerHostConfigSpec, sess *session.Session) {
	if !vch.UseVMGroup {
		return
	}

	exists, err := validate.VMGroupExists(trace.FromContext(ctx, ""), sess.Cluster, vch.VMGroupName)

	if err != nil {
		// #nosec: this method will not auto-escape HTML. Verify data is well formed.
		v.VCHIssues = template.HTML(fmt.Sprintf("%s<span class=\"error-message\">%s</span>\n", v.VCHIssues, html.EscapeString(err.Error())))
		return
	}

	if !exists {
		// #nosec: this method will not auto-escape HTML. Verify data is well formed.
		v.VCHIssues = template.HTML(fmt.Sprintf("%s<span class=\"error-message\">VCH is configured to use DRS VM Group %q, which cannot be found</span>\n", v.VCHIssues, html.EscapeString(vch.VMGroupName)))
	}

	return
}
