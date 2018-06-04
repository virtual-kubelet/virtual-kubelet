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

package vsphere

import (
	"io"
	"net/url"
	"os"

	"github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/pkg/trace"
)

func (i *ImageStore) Import(op trace.Operation, id string, spec *archive.FilterSpec, tarStream io.ReadCloser) error {
	l, err := i.NewDataSink(op, id)
	if err != nil {
		return err
	}

	return l.Import(op, spec, tarStream)
}

// NewDataSink creates and returns an DataSource associated with image storage
func (i *ImageStore) NewDataSink(op trace.Operation, id string) (storage.DataSink, error) {
	uri, err := i.URL(op, id)
	if err != nil {
		return nil, err
	}

	// there is no online fail over path for images
	// we should probably have a check in here as to whether the image is "sealed" and can no longer
	// be modified.
	return i.newDataSink(op, uri)
}

func (i *ImageStore) newDataSink(op trace.Operation, url *url.URL) (storage.DataSink, error) {
	mountPath, cleanFunc, err := i.Mount(op, url, true)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(mountPath)
	if err != nil {
		cleanFunc()
		return nil, err
	}

	return &storage.MountDataSink{
		Path:  f,
		Clean: cleanFunc,
	}, nil
}
