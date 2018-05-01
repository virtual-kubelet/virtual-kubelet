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

package image

import (
	"fmt"
	"net/url"
	"path"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/lib/portlayer/storage/volume"
	"github.com/vmware/vic/pkg/trace"
)

func Join(op trace.Operation, handle *exec.Handle, id, imgID, repoName string, img *Image) (*exec.Handle, error) {
	defer trace.End(trace.Begin(img.ID, op))

	// if _, ok := handle.ExecConfig.Mounts[volume.ID]; ok {
	// 	return nil, fmt.Errorf("Volume with ID %s is already in container %s's mountspec config", volume.ID, handle.ExecConfig.ID)
	// }

	// //constuct MountSpec for the tether
	// mountSpec := createMountSpec(volume, mountPath, diskOpts)
	// //append a device addition spec change to the container config
	// diskDevice := createVolumeVirtualDisk(volume)
	// config := createDeviceConfigSpec(diskDevice)
	// handle.Spec.DeviceChange = append(handle.Spec.DeviceChange, config)

	// if handle.ExecConfig.Mounts == nil {
	// 	handle.ExecConfig.Mounts = make(map[string]executor.MountSpec)
	// }
	// handle.ExecConfig.Mounts[volume.ID] = mountSpec

	// NOTE: from lib/spec/disk.go

	// set the rw layer name
	// NOTE: this is a POOR assumption - I'm not clear on how it's functioning on vSAN at all in shipping code given the assumption that
	// "[ds] id/id.vmdk" is a legitimate path. Some vsphere magic path adjustment?
	rwlayer := fmt.Sprintf("%s/%s.vmdk", path.Dir(handle.Spec.VMPathName()), id)

	disk := handle.Guest.NewDisk()
	moref := handle.Spec.Datastore.Reference()

	// NOTE: this spec construction really should be captured in one place down in the disk layer. That code is currently biased towards
	// the appliance disk flows so couples spec creation with disk creation/attach.
	// TODO: we absolutely shouldn't be mixing the handle.Spec.Datastore (wtf does this come from) and the DatastorePath for the disk
	disk.GetVirtualDevice().Backing = &types.VirtualDiskFlatVer2BackingInfo{
		DiskMode:        string(types.VirtualDiskModePersistent),
		ThinProvisioned: types.NewBool(true),

		VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
			FileName:  rwlayer,
			Datastore: &moref,
		},

		Parent: &types.VirtualDiskFlatVer2BackingInfo{
			VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
				FileName: img.DatastorePath.String(),
			},
		},
	}

	handle.Spec.AddVirtualDisk(disk)

	// record the repo name and image ID that resolved to the layer in question
	// NOTE: these really shouldn't be recorded directly like this, and are 1:1 with the image, not with the ExecConfig.
	// I suspect there's some tech-debt reason they got dropped into the main configuration like this.
	// I do recall that the repoName at least was recorded because many names/tags can point to the same layer so it's the
	// point-and-time-of-use name that we're recording. I assume the same is true for the imageID whereas the layerID is actually
	// stable
	handle.ExecConfig.LayerID = img.ID
	handle.ExecConfig.ImageID = imgID
	handle.ExecConfig.RepoName = repoName

	return handle, nil
}

func createVolumeVirtualDisk(volume *volume.Volume) *types.VirtualDisk {
	unitNumber := int32(-1)
	diskDevice := &types.VirtualDisk{
		CapacityInKB: 0,
		VirtualDevice: types.VirtualDevice{
			Key:           -1,
			ControllerKey: 100, //FIXME: This is hardcoded for now and should be located from the config spec in the future.
			UnitNumber:    &unitNumber,
			Backing: &types.VirtualDiskFlatVer2BackingInfo{
				DiskMode: string(types.VirtualDiskModeIndependent_persistent),
				VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
					FileName: volume.Device.DiskPath().Path,
				},
			},
		},
	}
	return diskDevice
}

func createDeviceConfigSpec(diskDevice *types.VirtualDisk) *types.VirtualDeviceConfigSpec {
	config := &types.VirtualDeviceConfigSpec{
		Device:        diskDevice,
		Operation:     types.VirtualDeviceConfigSpecOperationAdd,
		FileOperation: "", //blank for existing disk
	}
	return config
}

func createMountSpec(volume *volume.Volume, mountPath string, diskOpts map[string]string) executor.MountSpec {
	deviceMode := diskOpts[constants.Mode]
	newMountSpec := executor.MountSpec{
		Source: url.URL{
			Scheme: "label",
			Path:   volume.Label,
		},
		Path:     mountPath,
		Mode:     deviceMode,
		CopyMode: volume.CopyMode,
	}
	return newMountSpec
}
