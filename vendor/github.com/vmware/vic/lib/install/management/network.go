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

package management

import (
	"fmt"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
)

func (d *Dispatcher) createBridgeNetwork(conf *config.VirtualContainerHostConfigSpec) error {
	defer trace.End(trace.Begin("", d.op))

	// if the bridge network is already extant there's nothing to do
	bnet := conf.ExecutorConfig.Networks[conf.BridgeNetwork]
	if bnet != nil && bnet.ID != "" {
		return nil
	}

	// network didn't exist during validation given we don't have a moref, so create it
	if d.session.Client.IsVC() {
		// double check
		return errors.New("bridge network must already exist for vCenter environments")
	}

	// in this case the name to use is held in container network ID
	name := bnet.Network.ID

	d.op.Infof("Creating VirtualSwitch")
	hostNetSystem, err := d.session.Host.ConfigManager().NetworkSystem(d.op)
	if err != nil {
		err = errors.Errorf("Failed to retrieve host network system: %s", err)
		return err
	}

	if err = hostNetSystem.AddVirtualSwitch(d.op, name, &types.HostVirtualSwitchSpec{
		NumPorts: 1024,
	}); err != nil {
		err = errors.Errorf("Failed to add virtual switch (%q): %s", name, err)
		return err
	}

	d.op.Infof("Creating Portgroup")
	if err = hostNetSystem.AddPortGroup(d.op, types.HostPortGroupSpec{
		Name:        name,
		VlanId:      1, // TODO: expose this for finer grained grouping within the switch
		VswitchName: name,
		Policy:      types.HostNetworkPolicy{},
	}); err != nil {
		err = errors.Errorf("Failed to add port group (%q): %s", name, err)
		return err
	}

	net, err := d.session.Finder.Network(d.op, name)
	if err != nil {
		_, ok := err.(*find.NotFoundError)
		if !ok {
			err = errors.Errorf("Failed to query virtual switch (%q): %s", name, err)
			return err
		}
	}

	// assign the moref to the bridge network config on the appliance
	bnet.ID = net.Reference().String()
	bnet.Network.ID = net.Reference().String()
	conf.CreateBridgeNetwork = true
	d.op.Debugf("Created portgroup %q: %s", name, net)
	return nil
}

func (d *Dispatcher) removeNetwork(conf *config.VirtualContainerHostConfigSpec) error {
	defer trace.End(trace.Begin(conf.Name, d.op))

	if d.session.IsVC() {
		d.op.Debugf("Remove network is not supported for vCenter")
		return nil
	}
	if !conf.CreateBridgeNetwork {
		d.op.Infof("Bridge network was not created during VCH deployment, leaving it there")
		return nil
	}

	br := conf.ExecutorConfig.Networks["bridge"]
	if br == nil {
		return fmt.Errorf("Bridge Network ID is unknown")
	}
	name := br.Network.ID
	d.op.Debugf("Remove bridge network based on %s", name)

	moref := types.ManagedObjectReference{}
	ok := moref.FromString(name)
	if !ok {
		return fmt.Errorf("Unable to delete port group - failed to get moref from: %q", name)
	}

	net, err := d.session.Finder.ObjectReference(d.op, moref)
	if err != nil {
		return fmt.Errorf("Unable to delete port group - failed to find network from: %q", name)
	}
	d.op.Debugf("Delete bridge network: %s", net)

	netw, ok := net.(*object.Network)
	if !ok {
		d.op.Errorf("Expected Network Type, got %#v", net)
		return fmt.Errorf("Failed to get network for %q", moref)
	}
	pgName := netw.Name()

	hostNetSystem, err := d.session.Host.ConfigManager().NetworkSystem(d.op)
	if err != nil {
		return err
	}

	d.op.Infof("Removing Portgroup %q", pgName)
	err = hostNetSystem.RemovePortGroup(d.op, pgName)
	if err != nil {
		return err
	}

	d.op.Infof("Removing VirtualSwitch %q", pgName)
	err = hostNetSystem.RemoveVirtualSwitch(d.op, pgName)
	if err != nil {
		return err
	}
	return nil
}
