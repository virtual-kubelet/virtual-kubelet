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

package attach

import (
	"fmt"
	"net"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/migration/feature"
	"github.com/vmware/vic/lib/portlayer/exec"

	log "github.com/Sirupsen/logrus"
)

func lookupVCHIP() (net.IP, error) {
	// FIXME: THERE MUST BE ANOTHER WAY
	// following is from Create@exec.go
	ips, err := net.LookupIP(constants.ManagementHostName)
	if err != nil {
		return nil, err
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("No IP found on %s", constants.ManagementHostName)
	}

	if len(ips) > 1 {
		return nil, fmt.Errorf("Multiple IPs found on %s: %#v", constants.ManagementHostName, ips)
	}
	return ips[0], nil
}

func toggle(handle *exec.Handle, id string, connected bool) (*exec.Handle, error) {
	// check to see whether id is in Execs, if so set its RunBlock property to connected
	session, ok := handle.ExecConfig.Execs[id]
	if ok {
		if err := compatible(handle); err != nil {
			return nil, err
		}

		if session.Attach {
			session.RunBlock = connected
		}
	}

	// get the virtual device list
	devices := object.VirtualDeviceList(handle.Config.Hardware.Device)

	// select the virtual serial ports
	serials := devices.SelectByBackingInfo((*types.VirtualSerialPortURIBackingInfo)(nil))
	if len(serials) == 0 {
		return nil, fmt.Errorf("Unable to find a device with desired backing")
	}
	if len(serials) > 1 {
		return nil, fmt.Errorf("Multiple matches found with desired backing")
	}
	serial := serials[0]

	ip, err := lookupVCHIP()
	if err != nil {
		return nil, err
	}

	log.Debugf("Found a device with desired backing: %#v", serial)

	c := serial.GetVirtualDevice().Connectable
	b := serial.GetVirtualDevice().Backing.(*types.VirtualSerialPortURIBackingInfo)

	serviceURI := fmt.Sprintf("tcp://127.0.0.1:%d", constants.AttachServerPort)
	proxyURI := fmt.Sprintf("telnet://%s:%d", ip, constants.SerialOverLANPort)

	if b.ProxyURI == proxyURI && c.Connected == connected {
		log.Debugf("Already in the desired state, (connected: %t, proxyURI: %s)", connected, proxyURI)
		return handle, nil
	}

	// set the values
	log.Debugf("Setting Connected to %t", connected)
	c.Connected = connected
	if connected && handle.ExecConfig.Sessions[handle.ExecConfig.ID].Attach {
		log.Debugf("Setting the start connected state to %t", connected)
		c.StartConnected = handle.ExecConfig.Sessions[handle.ExecConfig.ID].Attach
	}

	log.Debugf("Setting ServiceURI to %s", serviceURI)
	b.ServiceURI = serviceURI

	log.Debugf("Setting the ProxyURI to %s", proxyURI)
	b.ProxyURI = proxyURI

	config := &types.VirtualDeviceConfigSpec{
		Device:    serial,
		Operation: types.VirtualDeviceConfigSpecOperationEdit,
	}
	handle.Spec.DeviceChange = append(handle.Spec.DeviceChange, config)

	// check to see whether id is in Sessions, if so set its RunBlock property to connected
	// if attach happens before start then this property will be persist in the vmx
	// if attach happens after start then this propery will be thrown away by commit (one cannot change persistent extraconfig values if the vm is powered on)
	session, ok = handle.ExecConfig.Sessions[id]
	if ok {
		if session.Attach {
			session.RunBlock = connected
		}
	}

	return handle, nil
}

func compatible(h interface{}) error {
	if handle, ok := h.(*exec.Handle); ok {
		if handle.DataVersion < feature.ExecSupportedVersion {
			return fmt.Errorf("attaching exec tasks not supported for this container")
		}

		return nil
	}

	return fmt.Errorf("Type assertion failed for %#+v", h)
}
