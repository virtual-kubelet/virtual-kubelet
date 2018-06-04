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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

const (
	// The duration waitForPath will tolerate before timing out.
	// TODO FIXME see GH issues 2340 and 2385
	// TODO We need to add a vSphere cancellation step to cancel calls that are taking too long
	// TODO Remove these TODOs after 2385 is completed
	pathTimeout = 60 * time.Second
)

// scsiScan tells the kernel to rescan the scsi bus.
func scsiScan() error {
	root := "/sys/class/scsi_host"

	dirs, err := ioutil.ReadDir(root)
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		file := path.Join(root, dir.Name(), "scan")
		// Channel, SCSI target ID, and LUN: "-" == rescan all
		err = ioutil.WriteFile(file, []byte("- - -"), 0)
		if err != nil {
			return err
		}
	}

	return nil
}

// Waits for a device to appear in the given directory and returns the
// resultant dev path (e.g /dev/sda). For instance, if sysPath is
// /sys/bus/pci/devices/0000:03:00.0/host0/subsystem/devices/0:0:0:0/block, the
// directory to appear in block will be mapped to /dev/<device>.  waitForDevice
// will wait for the entry to appear as a scsi target AND the blockdev to exist
// in /dev/, returning the path to /dev/<blockdev>.
func waitForDevice(op trace.Operation, sysPath string) (string, error) {
	defer trace.End(trace.Begin(sysPath))

	var err error
	op, _ = trace.WithTimeout(&op, time.Duration(pathTimeout), "waitForDevice(%s)", sysPath)
	errCh := make(chan error)

	var blockDev string
	go func() {
		t := time.NewTicker(200 * time.Microsecond)
		defer t.Stop()
		defer close(errCh)

		for range t.C {
			// We've timed out.
			if op.Err() != nil {
				return
			}

			// Syspath includes the scsi target itself.  Wait for it and try
			// again before trying to identify the device node it maps to.
			dirents, err := ioutil.ReadDir(sysPath)
			if err != nil {

				// try again
				if os.IsNotExist(err) {
					op.Debugf("Expected %s to appear. Trying again.", sysPath)
					continue
				}

				errCh <- err
				return
			}

			if len(dirents) > 1 {
				errCh <- fmt.Errorf("too many devices returned: %#v", dirents)
				return
			}

			if len(dirents) == 1 {
				blockDev = "/dev/" + dirents[0].Name()

				// check it exists
				if _, err := os.Stat(blockDev); err != nil {

					// try again
					if os.IsNotExist(err) {
						continue
					}

					errCh <- err
					return
				}

				// happy path
				return
			}

			// run a manual scan of the scsi bus
			if serr := scsiScan(); serr != nil {
				op.Warnf("scsi scan: %s", serr)
			}
		}
	}()

	op.Debugf("Waiting for attached disk to appear in %s, or timeout", sysPath)
	select {
	case err = <-errCh:

		if err != nil {
			return "", err
		}

		op.Infof("Attached disk present at %s", blockDev)
	case <-op.Done():
		if op.Err() != nil {
			return "", errors.Errorf("timeout waiting for layer to present in %s", sysPath)
		}
	}

	return blockDev, nil
}

// Ensures that a paravirtual scsi controller is present and determines the
// base path of disks attached to it returns a handle to the controller and a
// format string, with a single decimal for the disk unit number which will
// result in the /sys/bus/pci/devices/{pci id like
// 0000:03:00.0}/host{N provided by kernel}/subsystem/devices/N:0:{disk id like 0}:0/block/sd{Y provided by kernel} path.
// The directory inside block isn't a devnode, but it's name can be mapped to
// its /dev/ path.
func verifyParavirtualScsiController(op trace.Operation, vm *vm.VirtualMachine) (*types.ParaVirtualSCSIController, string, error) {
	devices, err := vm.Device(op)
	if err != nil {
		op.Errorf("vmware driver failed to retrieve device list for VM %s: %s", vm, errors.ErrorStack(err))
		return nil, "", errors.Trace(err)
	}

	controller, ok := devices.PickController((*types.ParaVirtualSCSIController)(nil)).(*types.ParaVirtualSCSIController)
	if controller == nil || !ok {
		err = errors.Errorf("vmware driver failed to find a paravirtual SCSI controller - ensure setup ran correctly")
		op.Errorf(err.Error())
		return nil, "", errors.Trace(err)
	}

	// build the base path
	// first we determine which label we're looking for (requires VMW hardware version >=10)
	controllerLabel := fmt.Sprintf("SCSI%d", controller.BusNumber)
	op.Debugf("Looking for scsi controller with label %s", controllerLabel)

	pciDevicesDir := "/sys/bus/pci/devices"
	pciBus, err := os.Open(pciDevicesDir)
	if err != nil {
		op.Errorf("Failed to open %s for reading: %s", pciDevicesDir, errors.ErrorStack(err))
		return controller, "", errors.Trace(err)
	}
	defer pciBus.Close()

	pciDevices, err := pciBus.Readdirnames(0)
	if err != nil {
		op.Errorf("Failed to read contents of %s: %s", pciDevicesDir, errors.ErrorStack(err))
		return controller, "", errors.Trace(err)
	}

	var controllerName string

	for _, pciDev := range pciDevices {
		labelPath := path.Join(pciDevicesDir, pciDev, "label")
		flabel, err := os.Open(labelPath)
		if err != nil {
			if !os.IsNotExist(err) {
				op.Errorf("Unable to read label from %s: %s", labelPath, errors.ErrorStack(err))
			}
			continue
		}
		defer flabel.Close()

		buf := make([]byte, len(controllerLabel))
		_, err = flabel.Read(buf)
		if err != nil {
			op.Errorf("Unable to read label from %s: %s", labelPath, errors.ErrorStack(err))
			continue
		}

		if controllerLabel == string(buf) {
			// we've found our controller
			controllerName = pciDev
			op.Debugf("Found pvscsi controller directory: %s", controllerName)

			break
		}
	}

	if controllerName == "" {
		err := errors.Errorf("Failed to locate pvscsi controller directory")
		return controller, "", errors.Trace(err)
	}

	// Use the block subsystem directly.
	// /sys/bus/pci/devices/0000:03:00.0/host0/subsystem/devices/0:0:0:0/block
	// hostN (host0 in this case) is provided to us by the kernel.
	// N:0:0:X where N is from above X is provided by vsphere

	// Glob for the scsi host
	matches, err := filepath.Glob(path.Join(pciDevicesDir, controllerName, "host*"))
	if err != nil {
		return controller, "", fmt.Errorf("scsi host glob failed: %s", err.Error())
	}

	if len(matches) != 1 {
		return controller, "", fmt.Errorf("too many scsi hosts")
	}

	// Get the number on the end
	hostStr := matches[0]
	hostidx := string(hostStr[len(hostStr)-1])

	// First param in the X:X:X:X path is the host id.  So `host2` will mean all devices will start with 2.
	formatString := path.Join(hostStr, fmt.Sprintf("subsystem/devices/%s:0:%%d:0/block/", hostidx))
	op.Debugf("Disk location format: %s", formatString)
	return controller, formatString, nil
}

func findDisk(op trace.Operation, vm *vm.VirtualMachine, filter func(diskName string, mode string) bool) ([]*types.VirtualDisk, error) {
	defer trace.End(trace.Begin(vm.String()))

	devices, err := vm.Device(op)
	if err != nil {
		return nil, fmt.Errorf("Failed to refresh devices for vm: %s", errors.ErrorStack(err))
	}

	candidates := devices.Select(func(device types.BaseVirtualDevice) bool {
		db := device.GetVirtualDevice().Backing
		if db == nil {
			return false
		}

		backing, ok := device.GetVirtualDevice().Backing.(*types.VirtualDiskFlatVer2BackingInfo)
		if !ok {
			return false
		}

		backingFileName := backing.VirtualDeviceFileBackingInfo.FileName
		mode := backing.DiskMode
		op.Debugf("backing file name %s, mode: %s", backingFileName, mode)

		return filter(backingFileName, mode)
	})

	if len(candidates) == 0 {
		return nil, nil
	}

	disks := make([]*types.VirtualDisk, len(candidates))
	for idx, disk := range candidates {
		disks[idx] = disk.(*types.VirtualDisk)
	}

	return disks, nil
}

// Find the disk by name attached to the given vm.
func findDiskByFilename(op trace.Operation, vm *vm.VirtualMachine, name string, persistent bool) (*types.VirtualDisk, error) {
	defer trace.End(trace.Begin(vm.String()))

	op.Debugf("Looking for attached disk matching filename %s", name)

	candidates, err := findDisk(op, vm, func(diskName string, mode string) bool {
		if persistent != (mode == string(types.VirtualDiskModePersistent) || mode == string(types.VirtualDiskModeIndependent_persistent)) {
			return false
		}

		match := strings.HasSuffix(diskName, name)
		if match {
			op.Debugf("Found candidate disk for %s at %s", name, diskName)
		}
		return match
	})

	if err != nil {
		op.Errorf("error finding disk: %s", err.Error())
		return nil, err
	}

	if len(candidates) == 0 {
		op.Infof("No disks match name and persistence: %s, %t", name, persistent)
		return nil, os.ErrNotExist
	}

	if len(candidates) > 1 {
		op.Errorf("Multiple disks match name: %s", name)
		// returning the first allows doing something with it
		return candidates[0], errors.Errorf("multiple disks match name: %s", name)
	}

	return candidates[0], nil
}

func findAllDisks(op trace.Operation, vm *vm.VirtualMachine) ([]*types.VirtualDisk, error) {
	defer trace.End(trace.Begin(vm.String()))

	op.Debugf("Looking for all attached disks")

	disks, err := findDisk(op, vm, func(diskName string, mode string) bool {
		return true
	})

	if err != nil {
		op.Errorf("error finding disk: %s", err.Error())
		return nil, err
	}

	return disks, nil
}
