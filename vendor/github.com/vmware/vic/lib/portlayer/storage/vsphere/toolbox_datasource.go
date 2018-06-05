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

package vsphere

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"os"

	"github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// ToolboxDataSource implements the DataSource interface for mounted devices
type ToolboxDataSource struct {
	ID    string
	VM    *vm.VirtualMachine
	Clean func()
}

// Source returns the data source associated with the DataSource
func (t *ToolboxDataSource) Source() interface{} {
	return t.VM
}

// Export reads data from the associated data source and returns it as a tar archive
func (t *ToolboxDataSource) Export(op trace.Operation, spec *archive.FilterSpec, data bool) (io.ReadCloser, error) {
	defer trace.End(trace.Begin("toolbox export"))

	client, err := GetToolboxClient(op, t.VM, t.ID)
	if err != nil {
		op.Errorf("Cannot get toolbox client: %s", err.Error())
		return nil, err
	}

	var readers []io.Reader
	for inclusion := range spec.Inclusions {
		// build a proper target
		target, err := BuildArchiveURL(op, t.ID, inclusion, spec, true, true)
		if err != nil {
			op.Errorf("Cannot build archive url: %s", err.Error())
			return nil, err
		}
		var tar io.ReadCloser
		var contentLength int64

		retryFunc := func() error {
			var retryErr error
			tar, contentLength, retryErr = client.Download(op, target)
			return retryErr
		}

		err = retry.DoWithConfig(retryFunc, isInvalidStateError, toolboxRetryConf)
		if err != nil {
			op.Errorf("Download error: %s", err.Error())
			return nil, err
		}
		op.Debugf("Downloaded from %s with size %d", target, contentLength)
		readers = append(readers, tar)

	}
	return ioutil.NopCloser(io.MultiReader(readers...)), nil
}

// Stat returns file stats of the destination header determined but the filterspec inclusion path
func (t *ToolboxDataSource) Stat(op trace.Operation, spec *archive.FilterSpec) (*storage.FileStat, error) {
	defer trace.End(trace.Begin("toolbox stat"))

	client, err := GetToolboxClient(op, t.VM, t.ID)
	if err != nil {
		op.Errorf("Cannot get toolbox client: %s", err.Error())
		return nil, err
	}

	// should only find a single path to stat, but make sure here.
	if len(spec.Inclusions) != 1 {
		op.Errorf("Stat called on more than one path: %+v", spec.Inclusions)
	}

	var statPath string
	inclusions := len(spec.Inclusions)
	if inclusions == 0 {
		op.Debugf("filter spec for stat operation has no inclusion specified : %#v", *spec)
	}

	if inclusions > 1 {
		op.Debugf("filter spec for stat operation had multiple inclusion paths : %#v", *spec)
	}

	for inclusion := range spec.Inclusions {
		statPath = inclusion
	}

	target, err := BuildArchiveURL(op, t.ID, statPath, spec, false, false)
	if err != nil {
		op.Errorf("Cannot build archive url: %s", err.Error())
		return nil, err
	}

	var statTar io.ReadCloser

	retryFunc := func() error {
		var retryErr error
		statTar, _, retryErr = client.Download(op, target)
		return retryErr
	}

	err = retry.DoWithConfig(retryFunc, isInvalidStateError, toolboxRetryConf)
	if err != nil {
		op.Errorf("Download error: %s", err.Error())
		return nil, err
	}
	defer statTar.Close()

	// decode from guest tools
	header, err := tar.NewReader(statTar).Next()
	if err == io.EOF {
		// special case - unable to get a single header translates to Not Found
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}

	stat := &storage.FileStat{
		Mode:       uint32(header.FileInfo().Mode()),
		Name:       header.Name,
		Size:       header.Size,
		ModTime:    header.ModTime,
		LinkTarget: header.Linkname,
	}

	return stat, nil
}

func (t *ToolboxDataSource) Close() error {
	t.Clean()

	return nil
}
