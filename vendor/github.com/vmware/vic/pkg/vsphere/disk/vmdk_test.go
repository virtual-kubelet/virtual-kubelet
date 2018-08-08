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

package disk

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/task"
	"github.com/vmware/govmomi/vim25/types"
)

func TestLockedDisks(t *testing.T) {
	lockErrJSON := "{\"FaultCause\":null,\"FaultMessage\":[{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"msg.fileio.lock\",\"Arg\":null,\"Message\":\"Failed to lock the file\"},{\"Key\":\"msg.disk.noBackEnd\",\"Arg\":[{\"Key\":\"1\",\"Value\":\"/vmfs/volumes/vsan:528032dc62774050-6ae24609f5c3816d/9d7c7159-c1b0-0579-1858-020023f9738b/volumes/bad81f01-8d32-4640-8b83-b4bb7e4a2350/bad81f01-8d32-4640-8b83-b4bb7e4a2350.vmdk\"}],\"Message\":\"Cannot open the disk '/vmfs/volumes/vsan:528032dc62774050-6ae24609f5c3816d/9d7c7159-c1b0-0579-1858-020023f9738b/volumes/bad81f01-8d32-4640-8b83-b4bb7e4a2350/bad81f01-8d32-4640-8b83-b4bb7e4a2350.vmdk' or one of the snapshot disks it depends on. \"},{\"Key\":\"msg.moduletable.powerOnFailed\",\"Arg\":[{\"Key\":\"1\",\"Value\":\"Disk\"}],\"Message\":\"Module 'Disk' power on failed. \"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.OpenFile.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of OpenFile[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.OpenFile.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of OpenFile[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.OpenFile.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of OpenFile[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.OpenFile.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of OpenFile[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.OpenFile.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of OpenFile[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.OpenFile.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of OpenFile[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"vob.fssvec.LookupAndOpen.file.failed\",\"Arg\":null,\"Message\":\"File system specific implementation of LookupAndOpen[file] failed\"},{\"Key\":\"msg.fileio.lock\",\"Arg\":null,\"Message\":\"Failed to lock the file\"},{\"Key\":\"msg.disk.noBackEnd\",\"Arg\":[{\"Key\":\"1\",\"Value\":\"/vmfs/volumes/vsan:528032dc62774050-6ae24609f5c3816d/9d7c7159-c1b0-0579-1858-020023f9738b/volumes/f1915d56-2c34-454f-bcdf-2a96db0600c2/f1915d56-2c34-454f-bcdf-2a96db0600c2.vmdk\"}],\"Message\":\"Cannot open the disk '/vmfs/volumes/vsan:528032dc62774050-6ae24609f5c3816d/9d7c7159-c1b0-0579-1858-020023f9738b/volumes/f1915d56-2c34-454f-bcdf-2a96db0600c2/f1915d56-2c34-454f-bcdf-2a96db0600c2.vmdk' or one of the snapshot disks it depends on. \"},{\"Key\":\"msg.vmx.poweron.failed\",\"Arg\":null,\"Message\":\"Failed to start the virtual machine.\"},{\"Key\":\"vpxd.vm.poweron.unexpectedfailure\",\"Arg\":[{\"Key\":\"1\",\"Value\":\"dazzling_almeida-db324aa3f47f\"}],\"Message\":\"An error was received from the ESX host while powering on VM dazzling_almeida-db324aa3f47f.\"}],\"Reason\":\"File system specific implementation of LookupAndOpen[file] failed\"}"
	nonLockErrJSON := "{\"FaultCause\":null,\"FaultMessage\":[{\"Key\":\"msg.vigor.operationCancelled\",\"Arg\":null,\"Message\":\"The operation was cancelled by the user.\"},{\"Key\":\"vpxd.vm.poweron.unexpectedfailure\",\"Arg\":[{\"Key\":\"1\",\"Value\":\"clever_brattain-f2d3325e3730\"}],\"Message\":\"An error was received from the ESX host while powering on VM clever_brattain-f2d3325e3730.\"}]}"

	tests := []struct {
		in            string
		lockedDevices []string
	}{
		{
			in: lockErrJSON,
			lockedDevices: []string{
				"/vmfs/volumes/vsan:528032dc62774050-6ae24609f5c3816d/9d7c7159-c1b0-0579-1858-020023f9738b/volumes/bad81f01-8d32-4640-8b83-b4bb7e4a2350/bad81f01-8d32-4640-8b83-b4bb7e4a2350.vmdk",
				"/vmfs/volumes/vsan:528032dc62774050-6ae24609f5c3816d/9d7c7159-c1b0-0579-1858-020023f9738b/volumes/f1915d56-2c34-454f-bcdf-2a96db0600c2/f1915d56-2c34-454f-bcdf-2a96db0600c2.vmdk",
			},
		},
		{
			in:            nonLockErrJSON,
			lockedDevices: nil,
		},
	}

	var vmFault types.GenericVmConfigFault
	for _, test := range tests {
		json.Unmarshal([]byte(test.in), &vmFault)
		err := task.Error{
			LocalizedMethodFault: &types.LocalizedMethodFault{
				Fault: &vmFault,
			},
		}

		devices := LockedDisks(err)
		assert.Equal(t, test.lockedDevices, devices, "didn't get expected locked devices")
	}
}
