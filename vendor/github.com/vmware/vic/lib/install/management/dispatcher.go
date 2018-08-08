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

package management

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"strings"
	"time"

	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/compute"
	"github.com/vmware/vic/pkg/vsphere/diagnostic"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// Action is the current action being performed
type Action int

// Action definitions
const (
	ActionConfigure Action = iota
	ActionCreate
	ActionDebug
	ActionDelete
	ActionInspect
	ActionInspectCertificates
	ActionInspectLogs
	ActionList
	ActionRollback
	ActionUpdate
	ActionUpgrade
)

// stringer for action
func (a Action) String() string {
	var act string
	switch a {
	case ActionConfigure:
		act = "configure"
	case ActionCreate:
		act = "create"
	case ActionDebug:
		act = "debug"
	case ActionDelete:
		act = "delete"
	case ActionInspect, ActionInspectCertificates, ActionInspectLogs:
		act = "inspect"
	case ActionList:
		act = "list"
	case ActionRollback:
		act = "rollback"
	case ActionUpdate:
		act = "update"
	case ActionUpgrade:
		act = "upgrade"
	}
	return act
}

type Dispatcher struct {
	Action

	session *session.Session
	op      trace.Operation
	force   bool
	secret  *extraconfig.SecretKey

	isVC          bool
	vchPoolPath   string
	vmPathName    string
	dockertlsargs string

	DockerPort string
	HostIP     string

	vchPool   *object.ResourcePool
	vchVapp   *object.VirtualApp
	appliance *vm.VirtualMachine

	oldApplianceISO string
	oldVCHResources *config.Resources

	sshEnabled         bool
	parentResourcepool *compute.ResourcePool
}

type diagnosticLog struct {
	key     string
	name    string
	start   int32
	host    *object.HostSystem
	collect bool
}

var diagnosticLogs = make(map[string]*diagnosticLog)

// NewDispatcher creates a dispatcher that can act upon VIC management operations.
// clientCert is an optional client certificate to allow interaction with the Docker API for verification
// force will ignore some errors
func NewDispatcher(ctx context.Context, s *session.Session, action Action, force bool) *Dispatcher {
	defer trace.End(trace.Begin(""))
	isVC := s.IsVC()
	e := &Dispatcher{
		Action:  action,
		session: s,
		op:      trace.FromContext(ctx, "Dispatcher"),
		isVC:    isVC,
		force:   force,
	}
	return e
}

// Get the current log header LineEnd of the hostd/vpxd logs based on VCH configuration
// With this we avoid collecting log file data that existed prior to install.
func (d *Dispatcher) InitDiagnosticLogsFromConf(conf *config.VirtualContainerHostConfigSpec) {
	defer trace.End(trace.Begin(""))

	if d.isVC {
		diagnosticLogs[d.session.ServiceContent.About.InstanceUuid] =
			&diagnosticLog{"vpxd:vpxd.log", "vpxd.log", 0, nil, true}
	}

	var err error
	// try best to get datastore and cluster, but do not return for any error. The least is to collect VC log only
	if d.session.Datastore == nil {
		if len(conf.ImageStores) > 0 {
			if d.session.Datastore, err = d.session.Finder.DatastoreOrDefault(d.op, conf.ImageStores[0].Host); err != nil {
				d.op.Debugf("Failure finding image store from VCH config (%s): %s", conf.ImageStores[0].Host, err.Error())
			} else {
				d.op.Debugf("Found ds: %s", conf.ImageStores[0].Host)
			}
		} else {
			d.op.Debug("Image datastore is empty")
		}
	}

	// find the host(s) attached to given storage
	if d.session.Cluster == nil {
		if len(conf.ComputeResources) > 0 {
			rp := compute.NewResourcePool(d.op, d.session, conf.ComputeResources[0])
			if d.session.Cluster, err = rp.GetCluster(d.op); err != nil {
				d.op.Debugf("Unable to get cluster for given resource pool %s: %s", conf.ComputeResources[0], err)
			}
		} else {
			d.op.Debug("Compute resource is empty")
		}
	}

	var hosts []*object.HostSystem
	if d.session.Datastore != nil && d.session.Cluster != nil {
		hosts, err = d.session.Datastore.AttachedClusterHosts(d.op, d.session.Cluster)
		if err != nil {
			d.op.Debugf("Unable to get the list of hosts attached to given storage: %s", err)
		}
	}

	if d.session.Host == nil {
		// vCenter w/ auto DRS.
		// Set collect=false here as we do not want to collect all hosts logs,
		// just the hostd log where the VM is placed.
		for _, host := range hosts {
			diagnosticLogs[host.Reference().Value] =
				&diagnosticLog{"hostd", "hostd.log", 0, host, false}
		}
	} else {
		// vCenter w/ manual DRS or standalone ESXi
		var host *object.HostSystem
		if d.isVC {
			host = d.session.Host
		}

		diagnosticLogs[d.session.Host.Reference().Value] =
			&diagnosticLog{"hostd", "hostd.log", 0, host, true}
	}

	m := diagnostic.NewDiagnosticManager(d.session)

	for k, l := range diagnosticLogs {
		if l == nil {
			continue
		}
		// get LineEnd without any LineText
		h, err := m.BrowseLog(d.op, l.host, l.key, math.MaxInt32, 0)
		if err != nil {
			d.op.Warnf("Disabling %s %s collection (%s)", k, l.name, err)
			diagnosticLogs[k] = nil
			continue
		}

		l.start = h.LineEnd
	}
}

// Get the current log header LineEnd of the hostd/vpxd logs based on vch VM hardwares, cause VCH configuration might not be available at this time
// With this we avoid collecting log file data that existed prior to install.
func (d *Dispatcher) InitDiagnosticLogsFromVCH(vch *vm.VirtualMachine) {
	defer trace.End(trace.Begin(""))

	if d.isVC {
		diagnosticLogs[d.session.ServiceContent.About.InstanceUuid] =
			&diagnosticLog{"vpxd:vpxd.log", "vpxd.log", 0, nil, true}
	}

	var err error
	// where the VM is running
	ds, err := d.getImageDatastore(vch, nil, true)
	if err != nil {
		d.op.Debugf("Failure finding image store from VCH VM %s: %s", vch.Reference(), err.Error())
	}

	var hosts []*object.HostSystem
	if ds != nil && d.session.Cluster != nil {
		hosts, err = ds.AttachedClusterHosts(d.op, d.session.Cluster)
		if err != nil {
			d.op.Debugf("Unable to get the list of hosts attached to given storage: %s", err)
		}
	}

	for _, host := range hosts {
		diagnosticLogs[host.Reference().Value] =
			&diagnosticLog{"hostd", "hostd.log", 0, host, false}
	}

	m := diagnostic.NewDiagnosticManager(d.session)

	for k, l := range diagnosticLogs {
		if l == nil {
			continue
		}
		// get LineEnd without any LineText
		h, err := m.BrowseLog(d.op, l.host, l.key, math.MaxInt32, 0)

		if err != nil {
			d.op.Warnf("Disabling %s %s collection (%s)", k, l.name, err)
			diagnosticLogs[k] = nil
			continue
		}

		l.start = h.LineEnd
	}
}

func (d *Dispatcher) CollectDiagnosticLogs() {
	defer trace.End(trace.Begin(""))

	m := diagnostic.NewDiagnosticManager(d.session)

	for k, l := range diagnosticLogs {
		if l == nil || !l.collect {
			continue
		}

		d.op.Infof("Collecting %s %s", k, l.name)

		var lines []string
		start := l.start

		for i := 0; i < 2; i++ {
			h, err := m.BrowseLog(d.op, l.host, l.key, start, 0)
			if err != nil {
				d.op.Errorf("Failed to collect %s %s: %s", k, l.name, err)
				break
			}

			lines = h.LineText
			if len(lines) != 0 {
				break // l.start was still valid, log was not rolled over
			}

			// log rolled over, start at the beginning.
			// TODO: If this actually happens we will have missed some log data,
			// it is possible to get data from the previous log too.
			start = 0
			d.op.Infof("%s %s rolled over", k, l.name)
		}

		if len(lines) == 0 {
			d.op.Warnf("No log data for %s %s", k, l.name)
			continue
		}

		f, err := os.Create(l.name)
		if err != nil {
			d.op.Errorf("Failed to create local %s: %s", l.name, err)
			continue
		}
		defer f.Close()

		for _, line := range lines {
			fmt.Fprintln(f, line)
		}
	}
}

func (d *Dispatcher) opManager(vch *vm.VirtualMachine) (*guest.ProcessManager, error) {
	state, err := vch.PowerState(d.op)
	if err != nil {
		return nil, fmt.Errorf("Failed to get appliance power state, service might not be available at this moment.")
	}
	if state != types.VirtualMachinePowerStatePoweredOn {
		return nil, fmt.Errorf("VCH appliance is not powered on, state %s", state)
	}

	running, err := vch.IsToolsRunning(d.op)
	if err != nil || !running {
		return nil, errors.New("Tools are not running in the appliance, unable to continue")
	}

	manager := guest.NewOperationsManager(d.session.Client.Client, vch.Reference())
	processManager, err := manager.ProcessManager(d.op)
	if err != nil {
		return nil, fmt.Errorf("Unable to manage processes in appliance VM: %s", err)
	}
	return processManager, nil
}

// opManagerWait polls for state of the process with the given pid, waiting until the process has completed.
// The pid param must be one returned by ProcessManager.StartProgram.
func (d *Dispatcher) opManagerWait(op trace.Operation, pm *guest.ProcessManager, auth types.BaseGuestAuthentication, pid int64) (*types.GuestProcessInfo, error) {
	pids := []int64{pid}

	for {
		select {
		case <-time.After(time.Millisecond * 250):
		case <-op.Done():
			return nil, fmt.Errorf("opManagerWait(%d): %s", pid, op.Err())
		}

		procs, err := pm.ListProcesses(op, auth, pids)
		if err != nil {
			return nil, err
		}

		if len(procs) == 1 && procs[0].EndTime != nil {
			return &procs[0], nil
		}
	}
}

func (d *Dispatcher) CheckAccessToVCAPI(vch *vm.VirtualMachine, target string) (int64, error) {
	pm, err := d.opManager(vch)
	if err != nil {
		return -1, err
	}
	auth := types.NamePasswordAuthentication{}
	spec := types.GuestProgramSpec{
		ProgramPath: "test-vc-api",
		Arguments:   target,
	}
	pid, err := pm.StartProgram(d.op, &auth, &spec)
	if err != nil {
		return -1, err
	}

	info, err := d.opManagerWait(d.op, pm, &auth, pid)
	if err != nil {
		return -1, err
	}

	return int64(info.ExitCode), nil
}

// addrToUse given candidateIPs, determines an address in cert that resolves to
// a candidateIP - this address can be used as the remote address to connect to with
// cert to ensure that certificate validation is successful
// if none can be found, return empty string and an err
func addrToUse(op trace.Operation, candidateIPs []net.IP, cert *x509.Certificate, cas []byte) (string, error) {
	if cert == nil {
		return "", errors.New("unable to determine suitable address with nil certificate")
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		op.Warnf("Failed to load system cert pool: %s. Using empty pool.", err)
		pool = x509.NewCertPool()
	}
	pool.AppendCertsFromPEM(cas)

	// update target to use FQDN
	for _, ip := range candidateIPs {
		names, err := net.LookupAddr(ip.String())
		if err != nil {
			op.Debugf("Unable to perform reverse lookup of IP address %s: %s", ip, err)
		}

		// check all the returned names, and lastly the raw IP
		for _, n := range append(names, ip.String()) {
			opts := x509.VerifyOptions{
				Roots:   pool,
				DNSName: n,
			}

			_, err := cert.Verify(opts)
			if err == nil {
				// this identifier will work
				op.Debugf("Matched %q for use against host certificate", n)
				// trim '.' fqdn suffix if fqdn
				return strings.TrimSuffix(n, "."), nil
			}

			op.Debugf("Checked %q, no match for host certificate", n)
		}
	}

	// no viable address
	return "", errors.New("unable to determine viable address")
}

/// viableHostAddresses attempts to determine which possibles addresses in the certificate
// are viable from the current location.
// This will return all IP addresses - it attempts to validate DNS names via resolution.
// This does NOT check connectivity
func viableHostAddress(op trace.Operation, candidateIPs []net.IP, cert *x509.Certificate, cas []byte) (string, error) {
	if cert == nil {
		return "", fmt.Errorf("unable to determine suitable address with nil certificate")
	}

	op.Debug("Loading CAs for client auth")
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(cas)

	dnsnames := cert.DNSNames

	// assemble the common name and alt names
	ip := net.ParseIP(cert.Subject.CommonName)
	if ip != nil {
		candidateIPs = append(candidateIPs, ip)
	} else {
		// assume it's dns
		dnsnames = append([]string{cert.Subject.CommonName}, dnsnames...)
	}

	// turn the DNS names into IPs
	for _, n := range dnsnames {
		// see which resolve from here
		ips, err := net.LookupIP(n)
		if err != nil {
			op.Debugf("Unable to perform IP lookup of %q: %s", n, err)
		}
		// Allow wildcard names for later validation
		if len(ips) == 0 && !strings.HasPrefix(n, "*") {
			op.Debugf("Discarding name from viable set: %s", n)
			continue
		}

		candidateIPs = append(candidateIPs, ips...)
	}

	// always add all the altname IPs - we're not checking for connectivity
	candidateIPs = append(candidateIPs, cert.IPAddresses...)

	return addrToUse(op, candidateIPs, cert, cas)
}
