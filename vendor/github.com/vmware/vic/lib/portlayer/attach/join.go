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

package attach

import (
	"fmt"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/pkg/trace"
)

// Join adds network backed serial port to the caller and configures them
func Join(h interface{}) (interface{}, error) {
	defer trace.End(trace.Begin(""))

	handle, ok := h.(*exec.Handle)
	if !ok {
		return nil, fmt.Errorf("Type assertion failed for %#+v", handle)
	}

	// Tether serial port - backed by network
	serial := &types.VirtualSerialPort{
		VirtualDevice: types.VirtualDevice{
			Backing: &types.VirtualSerialPortURIBackingInfo{
				VirtualDeviceURIBackingInfo: types.VirtualDeviceURIBackingInfo{
					Direction: string(types.VirtualDeviceURIBackingOptionDirectionClient),
					ProxyURI:  fmt.Sprintf("telnet://0.0.0.0:%d", constants.SerialOverLANPort),
					// Set it to 0.0.0.0 during Join call, VCH IP will be set when we call Bind
					ServiceURI: fmt.Sprintf("tcp://127.0.0.1:%d", constants.AttachServerPort),
				},
			},
			Connectable: &types.VirtualDeviceConnectInfo{
				Connected:         false,
				StartConnected:    false,
				AllowGuestControl: true,
			},
		},
		YieldOnPoll: true,
	}
	config := &types.VirtualDeviceConfigSpec{
		Device:    serial,
		Operation: types.VirtualDeviceConfigSpecOperationAdd,
	}
	handle.Spec.DeviceChange = append(handle.Spec.DeviceChange, config)

	return handle, nil
}
