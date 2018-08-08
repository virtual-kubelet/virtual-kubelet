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
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/vmware/vic/pkg/trace"
)

// FilesystemType represents the filesystem in use by a virtual disk
type FilesystemType uint8

const (
	// Ext4 represents the ext4 file system
	Ext4 FilesystemType = iota + 1

	// Xfs represents the XFS file system
	Xfs

	// Ntfs represents the NTFS file system
	Ntfs

	// directory in which to perform the direct mount of disk for bind mount
	// to actual target
	diskBindBase = "/.filesystem-by-label/"

	// used to isolate applications from the lost+found in the root of ext4
	VolumeDataDir = "/.vic.vol.data"
)

// Filesystem defines the interface for handling an attached virtual disk
type Filesystem interface {
	Mkfs(op trace.Operation, devPath, label string) error
	SetLabel(op trace.Operation, devPath, labelName string) error
	Mount(op trace.Operation, devPath, targetPath string, options []string) error
	Unmount(op trace.Operation, path string) error
}

// Semaphore represents the number of references to a disk
type Semaphore struct {
	resource string
	refname  string
	count    uint64
}

// NewSemaphore creates and returns a Semaphore initialized to 0
func NewSemaphore(r, n string) *Semaphore {
	return &Semaphore{
		resource: r,
		refname:  n,
		count:    0,
	}
}

// Increment increases the reference count by one
func (r *Semaphore) Increment() uint64 {
	return atomic.AddUint64(&r.count, 1)
}

// Decrement decreases the reference count by one
func (r *Semaphore) Decrement() uint64 {
	return atomic.AddUint64(&r.count, ^uint64(0))
}

// Count returns the current reference count
func (r *Semaphore) Count() uint64 {
	return atomic.LoadUint64(&r.count)
}

// InUseError is returned when a detach is attempted on a disk that is
// still in use
type InUseError struct {
	error
}

// VirtualDisk represents a VMDK in the datastore, the device node it may be
// attached at (if it's attached), the mountpoint it is mounted at (if
// mounted), and other configuration.
type VirtualDisk struct {
	*VirtualDiskConfig

	// The device node the disk is attached to
	DevicePath string

	// The path on the filesystem this device is attached to.
	mountPath string
	// The options that the disk is currently mounted with.
	mountOpts string

	// To avoid attach/detach races, this lock serializes operations to the disk.
	l sync.Mutex

	mountedRefs *Semaphore

	attachedRefs *Semaphore
}

// NewVirtualDisk creates and returns a new VirtualDisk object associated with the
// given datastore formatted with the specified FilesystemType
func NewVirtualDisk(op trace.Operation, config *VirtualDiskConfig, disks map[uint64]*VirtualDisk) (*VirtualDisk, error) {
	if !strings.HasSuffix(config.DatastoreURI.String(), ".vmdk") {
		return nil, fmt.Errorf("%s doesn't have a vmdk suffix", config.DatastoreURI.String())
	}

	if d, ok := disks[config.Hash()]; ok {
		return d, nil
	}
	op.Debugf("Didn't find the disk %s in the DiskManager cache, creating it", config.DatastoreURI)

	uri := config.DatastoreURI.String()
	d := &VirtualDisk{
		VirtualDiskConfig: config,
		mountedRefs:       NewSemaphore(uri, "mount"),
		attachedRefs:      NewSemaphore(uri, "attach"),
	}
	disks[config.Hash()] = d

	return d, nil
}

func (d *VirtualDisk) setAttached(op trace.Operation, devicePath string) (err error) {
	if d.DevicePath == "" {
		// Question: what happens if this is called a second time with a different devicePath?
		d.DevicePath = devicePath
	}

	count := d.attachedRefs.Increment()
	op.Debugf("incremented attach count for %s: %d", d.DatastoreURI, count)

	return nil
}

func (d *VirtualDisk) canBeDetached() error {
	if !d.attached() {
		return fmt.Errorf("%s is already detached", d.DatastoreURI)
	}

	if d.mounted() {
		return fmt.Errorf("%s is mounted (%s)", d.DatastoreURI, d.mountPath)
	}

	if d.inUseByOther() {
		return fmt.Errorf("Detach skipped - %s is still in use", d.DatastoreURI)
	}

	return nil
}

func (d *VirtualDisk) setDetached(op trace.Operation, disks map[uint64]*VirtualDisk) {
	// we only call this when it's been detached, so always make the updates
	op.Debugf("Dropping %s from the DiskManager cache", d.DatastoreURI)
	d.DevicePath = ""
	delete(disks, d.Hash())
}

// Mkfs formats the disk with Filesystem and sets the disk label
func (d *VirtualDisk) Mkfs(op trace.Operation, labelName string) error {
	d.l.Lock()
	defer d.l.Unlock()

	if !d.attached() {
		return fmt.Errorf("%s isn't attached", d.DatastoreURI)
	}

	if d.mounted() {
		return fmt.Errorf("%s is still mounted (%s)", d.DatastoreURI, d.mountPath)
	}

	return d.Filesystem.Mkfs(op, d.DevicePath, labelName)
}

// SetLabel sets this disk's label
func (d *VirtualDisk) SetLabel(op trace.Operation, labelName string) error {
	d.l.Lock()
	defer d.l.Unlock()

	if !d.attached() {
		return fmt.Errorf("%s isn't attached", d.DatastoreURI)
	}

	return d.Filesystem.SetLabel(op, d.DevicePath, labelName)
}

func (d *VirtualDisk) attached() bool {
	return d.DevicePath != ""
}

// Attached returns true if this disk is attached, false otherwise
func (d *VirtualDisk) Attached() bool {
	d.l.Lock()
	defer d.l.Unlock()

	return d.attached()
}

func (d *VirtualDisk) attachedByOther() bool {
	return d.attachedRefs.Count() > 1
}

// AttachedByOther returns true if the attached references are > 1
func (d *VirtualDisk) AttachedByOther() bool {
	d.l.Lock()
	defer d.l.Unlock()

	return d.attachedByOther()
}

func (d *VirtualDisk) mountedByOther() bool {
	return d.mountedRefs.Count() > 1
}

// MountedByOther returns true if the mounted references are > 1
func (d *VirtualDisk) MountedByOther() bool {
	d.l.Lock()
	defer d.l.Unlock()

	return d.mountedByOther()
}

func (d *VirtualDisk) inUseByOther() bool {
	return d.mountedByOther() || d.attachedByOther()
}

// InUseByOther returns true if the disk is currently attached or
// mounted by someone else
func (d *VirtualDisk) InUseByOther() bool {
	d.l.Lock()
	defer d.l.Unlock()

	return d.inUseByOther()
}

// Mount attempts to mount this disk. A NOP occurs if the disk is already mounted
// It returns the path at which the disk is mounted
// Enhancement: allow provision of mount path and refcount for:
//   specific mount point and options
func (d *VirtualDisk) Mount(op trace.Operation, options []string) (string, error) {
	d.l.Lock()
	defer d.l.Unlock()

	op.Debugf("Mounting %s", d.DatastoreURI)

	if !d.attached() {
		err := fmt.Errorf("%s isn't attached", d.DatastoreURI)
		op.Error(err)
		return "", err
	}

	opts := strings.Join(options, ";")
	if !d.mounted() {
		mntpath, err := ioutil.TempDir("", "mnt")
		if err != nil {
			err := fmt.Errorf("unable to create mountpint: %s", err)
			op.Error(err)
			return "", err
		}

		// get mount source, disk is already mounted if this func returns without error
		mntsrc, err := d.getMountSource(op, options)
		if err != nil {
			op.Error(err)
			return "", err
		}

		// then mount it at the correct source
		if strings.HasSuffix(mntsrc, VolumeDataDir) {
			// append bind mount options if we are masking lost+found
			options = append(options, "bind")
		}

		if err = d.Filesystem.Mount(op, mntsrc, mntpath, options); err != nil {
			op.Errorf("Failed to mount disk: %s", err)
			return "", err
		}

		d.mountPath = mntpath
		d.mountOpts = opts
	} else {
		// basic santiy check for matching options - we don't want to share a r/o mount
		// if the request was for r/w. Ideally we'd just mount this at a different location with the
		// requested options but that requires separate ref counting.
		// TODO: support differing mount opts
		if d.mountOpts != opts {
			op.Errorf("Unable to use mounted disk due to differing options: %s != %s", d.mountOpts, opts)
			return "", fmt.Errorf("incompatible mount options for disk reuse")
		}
	}

	count := d.mountedRefs.Increment()
	op.Debugf("incremented mount count for %s: %d", d.mountPath, count)

	return d.mountPath, nil
}

// getMountSource mounts the disk rootfs, checks if it has volumeDataDir, if so it returns volumeDataDir
// as the mount source to mask the lost+found folder, otherwise it returns the device path
// NOTE: this mount should not be counted in the ref counts, bindTarget will be unmounted when disk detaches.
// TODO: if we support different mount opts, we can't use the same bindTarget anymore.
// need to assign each opt a different name, we can add a field in VirtualDisk that tracks bindTarget
func (d *VirtualDisk) getMountSource(op trace.Operation, options []string) (string, error) {
	// need to first mount the disk under the diskBindBase
	bindTarget := path.Join(diskBindBase, d.DevicePath)

	// sanity check to make sure previous bindTarget is cleaned up properly
	var e1, e2 error
	_, e1 = os.Stat(bindTarget)
	if e1 == nil {
		// bindTarget exists, check whether or not bindTarget is a mount point
		e2 = os.Remove(bindTarget)
	}

	// we don't want to remount under the same mountpoint, so we only mounts under the following cases
	// first case: bindTarget exists but not a mountpoint
	// second case: bindTarget doesn't exist
	if (e1 == nil && e2 == nil) || os.IsNotExist(e1) {
		// #nosec
		if err := os.MkdirAll(bindTarget, 0744); err != nil {
			err = fmt.Errorf("unable to create mount point %s: %s", bindTarget, err)
			op.Error(err)
			return "", err
		}
		if err := d.Filesystem.Mount(op, d.DevicePath, bindTarget, options); err != nil {
			op.Errorf("Failed to mount disk: %s", err)
			return "", err
		}
	}

	mntsrc := path.Join(bindTarget, VolumeDataDir)
	// if the volume contains a volumeDataDir directory then mount that instead of the root of the filesystem
	// if we cannot read it we go with the root of the filesystem
	_, err := os.Stat(mntsrc)
	if err != nil {
		if os.IsNotExist(err) {
			// if there's no such directory then revert to using the device directly
			op.Infof("No " + VolumeDataDir + " data directory in volume, mounting filesystem directly")
			mntsrc = d.DevicePath
		} else {
			return "", fmt.Errorf("unable to determine whether lost+found masking is required: %s", err)
		}
	}
	return mntsrc, nil
}

// Unmount attempts to unmount a virtual disk
func (d *VirtualDisk) Unmount(op trace.Operation) error {
	d.l.Lock()
	defer d.l.Unlock()

	if !d.mounted() {
		return fmt.Errorf("%s already unmounted", d.DatastoreURI)
	}

	count := d.mountedRefs.Decrement()
	op.Debugf("decremented mount count for %s: %d", d.mountPath, count)

	if count > 0 {
		return nil
	}

	// no more mount references to this disk, so actually unmount
	if err := d.Filesystem.Unmount(op, d.mountPath); err != nil {
		err := fmt.Errorf("failed to unmount disk: %s", err)
		op.Error(err)
		return err
	}

	// only remove the mount directory - if we've succeeded in the unmount there won't be anything in it
	// if we somehow get here and there is content we do NOT want to delete it
	if err := os.Remove(d.mountPath); err != nil {
		err := fmt.Errorf("failed to clean up mount point: %s", err)
		op.Error(err)
		return err
	}

	d.mountPath = ""

	// mountpath is cleaned, we need to clean up the bindTarget as well
	bindTarget := path.Join(diskBindBase, d.DevicePath)
	if err := d.Filesystem.Unmount(op, bindTarget); err != nil {
		return fmt.Errorf("failed to clean up actual mount point on device: %s", err)
	}

	// only remove the mount directory - if we've succeeded in the unmount there won't be anything in it
	// if we somehow get here and there is content we do NOT want to delete it
	if err := os.Remove(bindTarget); err != nil {
		err := fmt.Errorf("failed to clean up actual mount point: %s", err)
		return err
	}

	return nil
}

func (d *VirtualDisk) mountPathFn() (string, error) {
	if !d.mounted() {
		return "", fmt.Errorf("%s isn't mounted", d.DatastoreURI)
	}

	return d.mountPath, nil
}

// MountPath returns the path on which the virtual disk is mounted,
// or an error if the disk is not mounted
func (d *VirtualDisk) MountPath() (string, error) {
	d.l.Lock()
	defer d.l.Unlock()

	return d.mountPathFn()
}

// DiskPath returns a URL referencing the path of the virtual disk
// on the datastore
func (d *VirtualDisk) DiskPath() url.URL {
	d.l.Lock()
	defer d.l.Unlock()

	return url.URL{
		Scheme: "ds",
		Path:   d.DatastoreURI.String(),
	}
}

func (d *VirtualDisk) mounted() bool {
	return d.mountPath != ""
}

// Mounted returns true if the virtual disk is mounted, false otherwise
func (d *VirtualDisk) Mounted() bool {
	d.l.Lock()
	defer d.l.Unlock()

	return d.mounted()
}

func (d *VirtualDisk) canBeUnmounted() error {
	if !d.attached() {
		return fmt.Errorf("%s is detached", d.DatastoreURI)
	}

	if !d.mounted() {
		return fmt.Errorf("%s is unmounted", d.DatastoreURI)
	}

	return nil
}

func (d *VirtualDisk) setUmounted() error {
	if !d.mounted() {
		return fmt.Errorf("%s already unmounted", d.DatastoreURI)
	}

	d.mountPath = ""
	return nil
}
