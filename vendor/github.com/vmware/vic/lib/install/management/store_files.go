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

package management

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
)

const (
	volumeRoot = "volumes"
	dsScheme   = "ds"
)

func (d *Dispatcher) deleteImages(conf *config.VirtualContainerHostConfigSpec) error {
	defer trace.End(trace.Begin("", d.op))
	var errs []string

	d.op.Info("Removing image stores")

	for _, imageDir := range conf.ImageStores {
		imageDSes, err := d.session.Finder.DatastoreList(d.op, imageDir.Host)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		if len(imageDSes) != 1 {
			errs = append(errs, fmt.Sprintf("Found %d datastores with provided datastore path %s. Provided datastore path must identify exactly one datastore.",
				len(imageDSes),
				imageDir.String()))

			continue
		}

		// delete images subfolder
		imagePath := path.Join(imageDir.Path, constants.StorageParentDir)
		if _, err = d.deleteDatastoreFiles(imageDSes[0], imagePath, true); err != nil {
			errs = append(errs, err.Error())
		}

		// delete kvStores subfolder
		kvPath := path.Join(imageDir.Path, constants.KVStoreFolder)
		if _, err = d.deleteDatastoreFiles(imageDSes[0], kvPath, true); err != nil {
			errs = append(errs, err.Error())
		}

		dsPath, err := datastore.URLtoDatastore(&imageDir)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		children, err := d.getChildren(imageDSes[0], dsPath)
		if err != nil {
			if !types.IsFileNotFound(err) {
				errs = append(errs, err.Error())
			}
			continue
		}

		if len(children) == 0 {
			d.op.Debugf("Removing empty image store parent directory [%s] %s", imageDir.Host, imageDir.Path)
			if _, err = d.deleteDatastoreFiles(imageDSes[0], imageDir.Path, true); err != nil {
				errs = append(errs, err.Error())
			}
		} else {
			d.op.Debug("Image store parent directory not empty, leaving in place.")
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

func (d *Dispatcher) deleteParent(ds *object.Datastore, root string) (bool, error) {
	defer trace.End(trace.Begin("", d.op))

	// alway forcing delete images
	return d.deleteDatastoreFiles(ds, root, true)
}

func (d *Dispatcher) deleteDatastoreFiles(ds *object.Datastore, path string, force bool) (bool, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("path %q, force %t", path, force), d.op))

	if ds == nil {
		err := errors.Errorf("No datastore")
		return false, err
	}

	// refuse to delete everything on the datstore, ignore force
	if path == "" {
		// #nosec: Errors unhandled.
		dsn, _ := ds.ObjectName(d.op)
		msg := fmt.Sprintf("refusing to remove datastore files for path \"\" on datastore %q", dsn)
		return false, errors.New(msg)
	}

	var empty bool
	dsPath := ds.Path(path)

	res, err := d.lsFolder(ds, dsPath)
	if err != nil {
		if !types.IsFileNotFound(err) {
			err = errors.Errorf("Failed to browse folder %q: %s", dsPath, err)
			return empty, err
		}
		d.op.Debugf("Folder %q is not found", dsPath)
		empty = true
		return empty, nil
	}
	if len(res.File) > 0 && !force {
		d.op.Debugf("Folder %q is not empty, leave it there", dsPath)
		return empty, nil
	}

	m := ds.NewFileManager(d.session.Datacenter, true)
	if err = d.deleteFilesIteratively(m, ds, dsPath); err != nil {
		return empty, err
	}
	return true, nil
}

func (d *Dispatcher) isVSAN(ds *object.Datastore) bool {
	// #nosec: Errors unhandled.
	dsType, _ := ds.Type(d.op)

	return dsType == types.HostFileSystemVolumeFileSystemTypeVsan
}

func (d *Dispatcher) deleteFilesIteratively(m *object.DatastoreFileManager, ds *object.Datastore, dsPath string) error {
	defer trace.End(trace.Begin(dsPath, d.op))

	if d.isVSAN(ds) {
		// Get sorted result to make sure child files are listed ahead of their parent folder so we empty the folder before deleting it.
		// This behaviour is specifically for vSan, as vSan sometimes throws an error when deleting a folder that is not empty.
		res, err := d.getSortedChildren(ds, dsPath)
		if err != nil {
			if !types.IsFileNotFound(err) {
				err = errors.Errorf("Failed to browse sub folders %q: %s", dsPath, err)
				return err
			}
			d.op.Debugf("Folder %q is not found", dsPath)
			return nil
		}

		for _, path := range res {
			if err = d.deleteVMFSFiles(m, ds, path); err != nil {
				return err
			}
		}
	}

	return d.deleteVMFSFiles(m, ds, dsPath)
}

func (d *Dispatcher) deleteVMFSFiles(m *object.DatastoreFileManager, ds *object.Datastore, dsPath string) error {
	defer trace.End(trace.Begin(dsPath, d.op))

	for _, ext := range []string{"-delta.vmdk", "-flat.vmdk"} {
		if strings.HasSuffix(dsPath, ext) {
			// Skip backing files as Delete() will do so via DeleteVirtualDisk
			return nil
		}
	}

	if err := m.Delete(d.op, dsPath); err != nil {
		d.op.Debugf("Failed to delete %q: %s", dsPath, err)
	}
	return nil
}

// getChildren returns all children under datastore path in unsorted order. (see also getSortedChildren)
func (d *Dispatcher) getChildren(ds *object.Datastore, dsPath string) ([]string, error) {
	res, err := d.lsSubFolder(ds, dsPath)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, dir := range res.HostDatastoreBrowserSearchResults {
		for _, f := range dir.File {
			dsf, ok := f.(*types.FileInfo)
			if !ok {
				continue
			}
			result = append(result, path.Join(dir.FolderPath, dsf.Path))
		}
	}
	return result, nil
}

// getSortedChildren returns all children under datastore path in reversed order.
func (d *Dispatcher) getSortedChildren(ds *object.Datastore, dsPath string) ([]string, error) {
	result, err := d.getChildren(ds, dsPath)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(result)))
	return result, nil
}

func (d *Dispatcher) lsSubFolder(ds *object.Datastore, dsPath string) (*types.ArrayOfHostDatastoreBrowserSearchResults, error) {
	defer trace.End(trace.Begin(dsPath, d.op))

	spec := types.HostDatastoreBrowserSearchSpec{
		MatchPattern: []string{"*"},
	}

	b, err := ds.Browser(d.op)
	if err != nil {
		return nil, err
	}

	task, err := b.SearchDatastoreSubFolders(d.op, dsPath, &spec)
	if err != nil {
		return nil, err
	}

	info, err := task.WaitForResult(d.op, nil)
	if err != nil {
		return nil, err
	}

	res := info.Result.(types.ArrayOfHostDatastoreBrowserSearchResults)
	return &res, nil
}

func (d *Dispatcher) lsFolder(ds *object.Datastore, dsPath string) (*types.HostDatastoreBrowserSearchResults, error) {
	defer trace.End(trace.Begin(dsPath, d.op))

	spec := types.HostDatastoreBrowserSearchSpec{
		MatchPattern: []string{"*"},
	}

	b, err := ds.Browser(d.op)
	if err != nil {
		return nil, err
	}

	task, err := b.SearchDatastore(d.op, dsPath, &spec)
	if err != nil {
		return nil, err
	}

	info, err := task.WaitForResult(d.op, nil)
	if err != nil {
		return nil, err
	}

	res := info.Result.(types.HostDatastoreBrowserSearchResults)
	return &res, nil
}

func (d *Dispatcher) createVolumeStores(conf *config.VirtualContainerHostConfigSpec) error {
	defer trace.End(trace.Begin("", d.op))
	for _, url := range conf.VolumeLocations {

		// NFS volumestores need only make it into the config of the vch
		if url.Scheme != dsScheme {
			d.op.Debugf("Skipping nfs volume store for vic-machine creation operation : (%s)", url.String())
			continue
		}

		ds, err := d.session.Finder.Datastore(d.op, url.Host)
		if err != nil {
			return errors.Errorf("Could not retrieve datastore with host %q due to error %s", url.Host, err)
		}

		if url.Path == "/" || url.Path == "" {
			url.Path = constants.StorageParentDir
		}

		nds, err := datastore.NewHelper(d.op, d.session, ds, url.Path)
		if err != nil {
			return errors.Errorf("Could not create volume store due to error: %s", err)
		}
		// FIXME: (GitHub Issue #1301) this is not valid URL syntax and should be translated appropriately when time allows
		url.Path = nds.RootURL.String()
	}
	return nil
}

// returns # of removed stores
func (d *Dispatcher) deleteVolumeStoreIfForced(conf *config.VirtualContainerHostConfigSpec, volumeStores *DeleteVolumeStores) (removed int) {
	defer trace.End(trace.Begin("", d.op))
	removed = 0

	deleteVolumeStores := d.force || (volumeStores != nil && *volumeStores == AllVolumeStores)

	if !deleteVolumeStores {
		if len(conf.VolumeLocations) == 0 {
			return 0
		}

		dsVolumeStores := new(bytes.Buffer)
		nfsVolumeStores := new(bytes.Buffer)
		for label, url := range conf.VolumeLocations {
			switch url.Scheme {
			case common.DsScheme:
				dsVolumeStores.WriteString(fmt.Sprintf("\t%s: %s\n", label, url.Path))
			case common.NfsScheme:
				nfsVolumeStores.WriteString(fmt.Sprintf("\t%s: %s\n", label, url.Path))
			}
		}
		d.op.Warnf("Since --force was not specified, the following volume stores will not be removed. Use the vSphere UI or supplied nfs targets to delete content you do not wish to keep.\n vsphere volumestores:\n%s\n NFS volumestores:\n%s\n", dsVolumeStores.String(), nfsVolumeStores.String())
		return 0
	}

	d.op.Info("Removing volume stores")
	for label, url := range conf.VolumeLocations {

		// NOTE: We cannot remove nfs VolumeStores at vic-machine delete time. We are not guaranteed to be on the correct network for any of the nfs stores.
		if url.Scheme != dsScheme {
			d.op.Warnf("Cannot delete VolumeStore (%s). It may not be reachable by vic-machine and has been skipped by the delete process.", url.String())
			continue
		}

		// FIXME: url is being encoded by the portlayer incorrectly, so we have to convert url.Path to the right url.URL object
		dsURL, err := datastore.ToURL(url.Path)
		if err != nil {
			d.op.Warnf("Didn't receive an expected volume store path format: %q", url.Path)
			continue
		}

		if dsURL.Path == constants.StorageParentDir {
			dsURL.Path = path.Join(dsURL.Path, constants.VolumesDir)
		}

		d.op.Debugf("Provided datastore URL: %q", url.Path)
		d.op.Debugf("Parsed volume store path: %q", dsURL.Path)
		d.op.Infof("Deleting volume store %q on Datastore %q at path %q",
			label, dsURL.Host, dsURL.Path)

		datastores, err := d.session.Finder.DatastoreList(d.op, dsURL.Host)

		if err != nil {
			d.op.Errorf("Error finding datastore %q: %s", dsURL.Host, err)
			continue
		}
		if len(datastores) > 1 {
			foundDatastores := new(bytes.Buffer)
			for _, d := range datastores {
				foundDatastores.WriteString(fmt.Sprintf("\n%s\n", d.InventoryPath))
			}
			d.op.Errorf("Ambiguous datastore name (%q) provided. Results were: %q", dsURL.Host, foundDatastores)
			continue
		}

		datastore := datastores[0]
		if _, err := d.deleteDatastoreFiles(datastore, dsURL.Path, deleteVolumeStores); err != nil {
			d.op.Errorf("Failed to delete volume store %q on Datastore %q at path %q", label, dsURL.Host, dsURL.Path)
		} else {
			removed++
		}
	}
	return removed

}
