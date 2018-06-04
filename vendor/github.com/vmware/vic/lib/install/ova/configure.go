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

package ova

import (
	"context"
	"net"
	"net/url"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config/dynamic/admiral"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

const (
	// ManagedByKey defines the extension key to use in the ManagedByInfo of the OVA
	ManagedByKey = "com.vmware.vic"
	// ManagedByType defines the type to use in the ManagedByInfo of the OVA
	ManagedByType = "VicApplianceVM"
)

// ConfigureManagedByInfo takes sets the ManagedBy field for the VM specified by ovaURL
func ConfigureManagedByInfo(ctx context.Context, config *session.Config, ovaURL string) error {
	sess := session.NewSession(config)
	sess, err := sess.Connect(ctx)
	if err != nil {
		return err
	}

	v, err := getOvaVMByTag(ctx, sess, ovaURL)
	if err != nil {
		return err
	}

	log.Infof("Attempting to configure ManagedByInfo")
	err = configureManagedByInfo(ctx, sess, v)
	if err != nil {
		return err
	}

	log.Infof("Successfully configured ManagedByInfo")
	return nil
}

func configureManagedByInfo(ctx context.Context, sess *session.Session, v *vm.VirtualMachine) error {
	spec := types.VirtualMachineConfigSpec{
		ManagedBy: &types.ManagedByInfo{
			ExtensionKey: ManagedByKey,
			Type:         ManagedByType,
		},
	}

	info, err := v.WaitForResult(ctx, func(ctx context.Context) (tasks.Task, error) {
		return v.Reconfigure(ctx, spec)
	})

	if err != nil {
		log.Errorf("Error while setting ManagedByInfo: %s", err)
		return err
	}

	if info.State != types.TaskInfoStateSuccess {
		log.Errorf("Setting ManagedByInfo reported: %s", info.Error.LocalizedMessage)
		return err
	}

	return nil
}

func getOvaVMByTag(ctx context.Context, sess *session.Session, u string) (*vm.VirtualMachine, error) {
	ovaURL, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	host := ovaURL.Hostname()

	log.Debugf("Looking up host %s", host)
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, errors.Errorf("IP lookup failed: %s", err)
	}

	log.Debugf("found %d IP(s) from hostname lookup on %s:", len(ips), host)
	var ip string
	for _, i := range ips {
		log.Debugf(i.String())
		if i.To4() != nil {
			ip = i.String()
		}
	}

	if ip == "" {
		return nil, errors.Errorf("IPV6 support not yet implemented")
	}

	vms, err := admiral.DefaultDiscovery.Discover(ctx, sess)
	if err != nil {
		return nil, errors.Errorf("failed to discover OVA vm(s): %s", err)
	}

	log.Infof("Found %d VM(s) tagged as OVA", len(vms))
	for i, v := range vms {
		log.Debugf("Checking IP for %s", v.Reference().Value)
		vmIP, err := v.WaitForIP(ctx)
		if err != nil && i == len(vms)-1 {
			return nil, errors.Errorf("failed to get VM IP: %s", err)
		}

		// verify the tagged vm has the IP we expect
		if vmIP == ip {
			log.Debugf("Found OVA with matching IP: %s", ip)
			return v, nil
		}
	}

	return nil, errors.Errorf("no VM(s) found with OVA tag")
}
