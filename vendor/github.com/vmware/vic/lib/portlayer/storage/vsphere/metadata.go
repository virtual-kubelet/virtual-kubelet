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

package vsphere

import (
	"bytes"
	"io/ioutil"
	"path"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
)

// Write the opaque metadata blobs (by name).
// Each blob in the metadata map is written to a file with the corresponding
// name.  Likewise, when we read it back (on restart) we populate the map
// accordingly.
func WriteMetadata(op trace.Operation, ds *datastore.Helper, dir string, meta map[string][]byte) error {
	// XXX this should be done via disklib so this meta follows the disk in
	// case of motion.

	if meta != nil && len(meta) != 0 {
		for name, value := range meta {
			r := bytes.NewReader(value)
			pth := path.Join(dir, name)
			op.Infof("Writing metadata %s", pth)
			if err := ds.Upload(op, r, pth); err != nil {
				return err
			}
		}
	} else {
		if _, err := ds.Mkdir(op, false, dir); err != nil {
			return err
		}
	}

	return nil
}

// Read the metadata from the given dir
func GetMetadata(op trace.Operation, ds *datastore.Helper, dir string) (map[string][]byte, error) {

	res, err := ds.Ls(op, dir)
	if err != nil {
		return nil, err
	}

	if len(res.File) == 0 {
		op.Infof("No meta found for %s", dir)
		return nil, nil
	}

	meta := make(map[string][]byte)
	for _, f := range res.File {
		// we're only interested in files, not folders
		finfo, ok := f.(*types.FileInfo)
		if !ok {
			continue
		}

		p := path.Join(dir, finfo.Path)
		op.Infof("Getting metadata %s", p)
		rc, err := ds.Download(op, p)
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		buf, err := ioutil.ReadAll(rc)
		if err != nil {
			return nil, err
		}

		meta[finfo.Path] = buf
	}

	return meta, nil
}
