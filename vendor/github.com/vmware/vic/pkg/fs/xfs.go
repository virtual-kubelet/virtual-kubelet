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

package fs

import (
	"os/exec"
	"strings"

	"github.com/docker/docker/pkg/mount"

	"github.com/vmware/vic/pkg/trace"
)

// XFS satisfies the Filesystem interface
type XFS struct{}

// Create an XFS filesystem manager
func NewXFS() *XFS {
	return &XFS{}
}

// Mkfs creates an xfs fs on the given device and applices the given label
func (e *XFS) Mkfs(op trace.Operation, devPath, label string) error {
	defer trace.End(trace.Begin(devPath))

	op.Infof("Creating xfs filesystem on device %s", devPath)

	// #nosec: Subprocess launching with variable
	cmd := exec.Command("/sbin/mkfs.xfs", "-n", "ftype=1", "-L", label, devPath)

	if output, err := cmd.CombinedOutput(); err != nil {
		op.Errorf("vmdk storage driver failed to format disk %s: %s", devPath, err)
		op.Errorf("mkfs output: %s", string(output))
		return err
	}
	op.Debugf("Filesystem created on device %s", devPath)

	return nil
}

// Mount mounts an xfs formatted device at the given path.  From the Docker
// mount pkg, args must in the form arg=val.
func (e *XFS) Mount(op trace.Operation, devPath, targetPath string, options []string) error {
	defer trace.End(trace.Begin(devPath))
	op.Infof("Mounting %s to %s", devPath, targetPath)
	return mount.Mount(devPath, targetPath, "xfs", strings.Join(options, ","))
}

// Unmount unmounts the disk.
// path can be a device path or a mount point
func (e *XFS) Unmount(op trace.Operation, path string) error {
	defer trace.End(trace.Begin(path))
	op.Infof("Unmounting %s", path)
	return mount.Unmount(path)
}

// SetLabel sets the label of an xfs formated device
func (e *XFS) SetLabel(op trace.Operation, devPath, labelName string) error {
	defer trace.End(trace.Begin(devPath))

	// #nosec: Subprocess launching with variable
	cmd := exec.Command("/sbin/e2label", devPath, labelName)
	if output, err := cmd.CombinedOutput(); err != nil {
		op.Errorf("failed to set label %s: %s", devPath, err)
		op.Errorf(string(output))
		return err
	}

	return nil
}
