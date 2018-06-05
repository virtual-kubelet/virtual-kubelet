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
	"bytes"
	"io"

	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// ToolboxDataSink implements the DataSink interface for mounted devices
type ToolboxDataSink struct {
	ID    string
	VM    *vm.VirtualMachine
	Clean func()
}

// Sink returns the data sink associated with the DataSink
func (t *ToolboxDataSink) Sink() interface{} {
	return t.VM
}

// Import writes `data` to the data sink associated with this DataSink
func (t *ToolboxDataSink) Import(op trace.Operation, spec *archive.FilterSpec, data io.ReadCloser) error {
	defer trace.End(trace.Begin("toolbox import"))

	client, err := GetToolboxClient(op, t.VM, t.ID)
	if err != nil {
		op.Debugf("Cannot get toolbox client: %s", err.Error())
		return err
	}

	target, err := BuildArchiveURL(op, t.ID, spec.RebasePath, spec, true, true)
	if err != nil {
		op.Debugf("Cannot build archive url: %s", err.Error())
		return err
	}

	// buffer the data - needed to allow retry or the Upload drains the reader before the failure
	// and we lose the data
	// TODO: should look into chunking so that we can support copy of very large files.
	// NOW: need a check that size doesn't exceed available memory - and error recommending offline
	// copy as alternative
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, data)
	if err != nil {
		op.Errorf("Unable to buffer archive data for upload")
		return err
	}

	// upload the gzip archive.
	p := soap.DefaultUpload

	retryFunc := func() error {
		return client.Upload(op, buf, target, p, &types.GuestPosixFileAttributes{}, true)
	}

	err = retry.DoWithConfig(retryFunc, isInvalidStateError, toolboxRetryConf)

	if err != nil {
		op.Debugf("Upload error: %s", err.Error())
	}

	return err
}

func (t *ToolboxDataSink) Close() error {
	t.Clean()

	return nil
}
