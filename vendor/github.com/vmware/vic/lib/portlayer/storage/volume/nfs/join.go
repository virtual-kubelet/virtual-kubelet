// Copyright 2017-2018 VMware, Inc. All Rights Reserved.
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

package nfs

import (
	"fmt"

	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/lib/portlayer/storage/volume"
	"github.com/vmware/vic/pkg/trace"
)

const (
	//Below is a list of options included in the mount options and a brief reason why.
	// rw : on by default, for ro the syscall.MS_READONLY flag must be set instead of putting it in the options pointer of the mount call.
	// "noatime" : this option prevents read's from triggering a write to update the accesstime of an inode. Helps with efficient reads. (Specified as a flag during the tether operation.)
	// "vers=3" : we want to use NFSv3 it is simpler to implement and we do not need all the features of NFSv4 at this time
	// "rsize=131072" : indicates the maximum read size for data on the NFS server. If the rsize is too big for either the client or server a negotion will occur to determine a supportable size. the value chosen is a default for /bin/mount for ubuntu 16.04
	// "wsize=131072" : indicates the maximum write size for data on the NFS server. Like rsize it is negotiated if the value is too large. the value chosen is a default for /bin/mount for ubuntu 16.04
	// "hard" : implies that we will retry indefinitely upon a failed transmission of data. this was agreed upon to indicate that problems have occurred with the mount if a hang on a write occurs.
	// "proto=tcp" : tcp is a realiable protocol. The client used by the VCH also uses TCP as it's protocol. meaning we gain consistency in the communication between the tether->nfs and portlayer->nfs
	// "timeo=600" : 600 deciseconds. This means a 60 second timout. With "hard" enabled this option likely does not matter.
	// "sec=sys" : this means the NFS client uses the AUTH_SYS security flavor for all NFS requests on this mount point. This requires UID and GID of the user for permissions. also allows squashing permissions
	// "mountvers=3" : this is listed as the RPC bind version. However, it is listed as a default by /bin/mount even when RPC is not the protocol used.
	// "mountProto=TCP" : since the VCH uses TCP we should be using it as well here on the tether. Additionally, the mountProto does effect the initial protocol used for interacting with an nfs server. Keeping everything as the same protocol makes protocol issues easier to detect.
	nfsMountOptions = "vers=3,rsize=131072,wsize=131072,hard,proto=tcp,timeo=600,sec=sys,mountvers=3,mountproto=tcp,nolock"
)

func VolumeJoin(op trace.Operation, handle *exec.Handle, volume *volume.Volume, mountPath string, diskOpts map[string]string) (*exec.Handle, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("handle.ID(%s), volume(%s), mountPath(%s), diskPath(%#v)", handle.ExecConfig.ID, volume.ID, mountPath, volume.Device.DiskPath())))

	if _, ok := handle.ExecConfig.Mounts[volume.ID]; ok {
		return nil, fmt.Errorf("Volume with ID %s is already in container %s's mountspec config", volume.ID, handle.ExecConfig.ID)
	}

	// construct MountSpec for the tether
	mountSpec := createMountSpec(volume, mountPath, diskOpts)
	if handle.ExecConfig.Mounts == nil {
		handle.ExecConfig.Mounts = make(map[string]executor.MountSpec)
	}
	handle.ExecConfig.Mounts[volume.ID] = *mountSpec

	return handle, nil
}

func createMountSpec(volume *volume.Volume, mountPath string, diskOpts map[string]string) *executor.MountSpec {
	host := volume.Device.DiskPath()
	deviceMode := nfsMountOptions + ",addr=" + host.Host

	// Note: rw mode is not specified in the device node since the syscall.Mount defaults to rw. Additional, "noatime" must be indicated with the flag syscall.MS_NOATIME
	newMountSpec := executor.MountSpec{
		Source:   host,
		Path:     mountPath,
		Mode:     deviceMode,
		CopyMode: volume.CopyMode,
	}
	return &newMountSpec
}
