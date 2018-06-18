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

package vspc

import (
	"fmt"
	"net"

	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/pkg/telnet"
)

func isKnownSuboptions(cmd []byte) bool {
	if len(cmd) < 3 {
		return false
	}
	return cmd[0] == telnet.Sb && cmd[1] == VmwareExt && cmd[2] == KnownSuboptions1
}

func isDoProxy(cmd []byte) bool {
	if len(cmd) < 3 {
		return false
	}
	return cmd[0] == telnet.Sb && cmd[1] == VmwareExt && cmd[2] == DoProxy
}

func isVmotionBegin(cmd []byte) bool {
	if len(cmd) < 3 {
		return false
	}
	return cmd[0] == telnet.Sb && cmd[1] == VmwareExt && cmd[2] == VmotionBegin
}

func isVmotionPeer(cmd []byte) bool {
	if len(cmd) < 3 {
		return false
	}
	return cmd[0] == telnet.Sb && cmd[1] == VmwareExt && cmd[2] == VmotionPeer
}

func isVmotionComplete(cmd []byte) bool {
	if len(cmd) < 3 {
		return false
	}
	return cmd[0] == telnet.Sb && cmd[1] == VmwareExt && cmd[2] == VmotionComplete
}

func isVmotionAbort(cmd []byte) bool {
	if len(cmd) < 3 {
		return false
	}
	return cmd[0] == telnet.Sb && cmd[1] == VmwareExt && cmd[2] == VmotionAbort
}

func isVMName(cmd []byte) bool {
	if len(cmd) < 3 {
		return false
	}
	return cmd[0] == telnet.Sb && cmd[1] == VmwareExt && cmd[2] == VMName
}

func isVMUUID(cmd []byte) bool {
	if len(cmd) < 3 {
		return false
	}
	return cmd[0] == telnet.Sb && cmd[1] == VmwareExt && cmd[2] == VMVCUUID
}

func getVMUUID() []byte {
	return []byte{telnet.Iac, telnet.Sb, VmwareExt, GetVMVCUUID, telnet.Iac, telnet.Se}
}

func lookupVCHIP() (net.IP, error) {
	ips, err := net.LookupIP(constants.ManagementHostName)
	if err != nil {
		return nil, err
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no ip found on %s", constants.ManagementHostName)
	}

	if len(ips) > 1 {
		return nil, fmt.Errorf("multiple ips found on %s: %#v", constants.ManagementHostName, ips)
	}
	return ips[0], nil
}
