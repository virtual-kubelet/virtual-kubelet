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

package performance

import (
	"fmt"
	"strconv"
	"time"

	"github.com/docker/docker/pkg/pubsub"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/session"
)

// vmSubscription is a 1:1 relationship to a Virtual Machine and a
// 1:M relationship to subscribers
type vmSubscription struct {
	vm *object.VirtualMachine
	id string

	pub                 *pubsub.Publisher
	devices             object.VirtualDeviceList
	deviceInstanceToKey map[string]string

	diskNames    []string // container's virtualDisk names
	networkNames []string // container's network names
}

// DeviceName will return the name associated with the metric instance id
func (sub *vmSubscription) DeviceName(instance string) string {
	var name string

	// did we previously find this device
	if name, exists := sub.deviceInstanceToKey[instance]; exists {
		return name
	}

	// convert instance to key - we are expecting regular failures, so no logging
	// or returning of error
	key, err := strconv.Atoi(instance)
	if err != nil {
		// this is not a key, so return an empty string
		return name
	}

	// find the device and get the name
	device := sub.devices.FindByKey(int32(key))
	if device != nil {
		// get the name
		name = sub.devices.Name(device)
		// populate map
		sub.deviceInstanceToKey[instance] = name
	}
	return name
}

// ID returns the subscription's id
func (sub *vmSubscription) ID() string {
	return sub.id
}

// DeviceList retrieves the VMs devices and builds slices of the disk and network
// names
func (sub *vmSubscription) DeviceList(op trace.Operation) error {
	list, err := sub.vm.Device(op.Context)
	if err != nil {
		op.Errorf("vm stats subscription(%s) unable to load devices: %s", sub.ID(), err)
		return err
	}

	// populate slice for disk and network names
	for i := range list {
		switch list.Type(list[i]) {
		case object.DeviceTypeDisk:
			// disk names are presented by the performanceManager based on their relationship to the
			// controller.  So we need to create a disk name that aligns with the performance manager
			// naming pattern
			disk := list[i].GetVirtualDevice()
			switch c := list.FindByKey(disk.ControllerKey).(type) {
			case types.BaseVirtualSCSIController:
				sub.diskNames = append(sub.diskNames, fmt.Sprintf("%s%d:%d", "scsi", c.GetVirtualSCSIController().BusNumber, *disk.UnitNumber))
			}
		case object.DeviceTypeEthernet:
			sub.networkNames = append(sub.networkNames, list.Name(list[i]))
		}
	}

	sub.devices = list
	return nil
}

// Disks returns an initialized slice of the containers VirtualDisks
func (sub *vmSubscription) Disks() []VirtualDisk {
	var disks []VirtualDisk
	for i := range sub.diskNames {
		d := VirtualDisk{
			Name:  sub.diskNames[i],
			Read:  DiskUsage{},
			Write: DiskUsage{},
		}
		disks = append(disks, d)
	}
	return disks
}

// Networks returns an initialized slice of the containers networks
func (sub *vmSubscription) Networks() []Network {
	var networks []Network
	for i := range sub.networkNames {
		n := Network{
			Name: sub.networkNames[i],
			Rx:   NetworkUsage{},
			Tx:   NetworkUsage{},
		}
		networks = append(networks, n)
	}
	return networks
}

// Publish sends the metric to all channels subscribed
func (sub *vmSubscription) Publish(metric *VMMetrics) {
	// if no disk / network reported then add the defaults
	if len(metric.Disks) == 0 {
		metric.Disks = sub.Disks()
	}
	if len(metric.Networks) == 0 {
		metric.Networks = sub.Networks()
	}
	sub.pub.Publish(metric)
}

// Publishers returns the number of channels subscribed to the container
func (sub *vmSubscription) Publishers() int {
	return sub.pub.Len()
}

// Channel provides the communication chan for metrics subscriptions
func (sub *vmSubscription) Channel() chan interface{} {
	return sub.pub.Subscribe()
}

// Evict will remove the channel from the publisher ending the subscription
func (sub *vmSubscription) Evict(ch chan interface{}) {
	sub.pub.Evict(ch)
}

// newVMSubscription is a helper func to convert the interface to a subscription
func newVMSubscription(op trace.Operation, session *session.Session, moref types.ManagedObjectReference, id string) (*vmSubscription, error) {
	// ensure we have a valid moRef..we won't worry about inspecting the details
	if moref.String() == "" {
		err := fmt.Errorf("no vm associated with new stats subscription: %s", id)
		op.Errorf("%s", err)
		return nil, err
	}

	sub := &vmSubscription{
		vm:                  object.NewVirtualMachine(session.Vim25(), moref),
		deviceInstanceToKey: make(map[string]string),
	}

	err := sub.DeviceList(op)
	if err != nil {
		return nil, err
	}

	// create the publisher
	sub.pub = pubsub.NewPublisher(100*time.Millisecond, 0)
	return sub, nil
}
