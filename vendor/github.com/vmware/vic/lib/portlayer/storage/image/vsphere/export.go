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
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/pkg/trace"
)

// Export reads the delta between child and parent image layers, returning
// the difference as a tar archive.
//
// id - must inherit from ancestor if ancestor is specified
// ancestor - the layer up the chain against which to diff
// spec - describes filters on paths found in the data (include, exclude, rebase, strip)
// data - set to true to include file data in the tar archive, false to include headers only
func (i *ImageStore) Export(op trace.Operation, id, ancestor string, spec *archive.FilterSpec, data bool) (io.ReadCloser, error) {
	l, err := i.NewDataSource(op, id)
	if err != nil {
		return nil, err
	}

	if ancestor == "" {
		return l.Export(op, spec, data)
	}

	// for now we assume ancestor instead of entirely generic left/right
	// this allows us to assume it's an image
	r, err := i.NewDataSource(op, ancestor)
	if err != nil {
		op.Debugf("Unable to get datasource for ancestor: %s", err)

		l.Close()
		return nil, err
	}

	closers := func() error {
		op.Debugf("Callback to io.Closer function for image delta export")

		l.Close()
		r.Close()

		return nil
	}

	ls := l.Source()
	rs := r.Source()

	fl, lok := ls.(*os.File)
	fr, rok := rs.(*os.File)

	if !lok || !rok {
		go closers()
		return nil, fmt.Errorf("mismatched datasource types: %T, %T", ls, rs)
	}

	// if we want data, exclude the xattrs, otherwise assume diff
	tar, err := archive.Diff(op, fl.Name(), fr.Name(), spec, data, !data)
	if err != nil {
		go closers()
		return nil, err
	}

	return &storage.ProxyReadCloser{
		ReadCloser: tar,
		Closer:     closers,
	}, nil
}

func (i *ImageStore) NewDataSource(op trace.Operation, id string) (storage.DataSource, error) {
	url, err := i.URL(op, id)
	if err != nil {
		return nil, err
	}

	return i.newDataSource(op, url)
}

func (i *ImageStore) newDataSource(op trace.Operation, url *url.URL) (storage.DataSource, error) {
	mountPath, cleanFunc, err := i.Mount(op, url, false)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(mountPath)
	if err != nil {
		cleanFunc()
		return nil, err
	}

	op.Debugf("Created mount data source for access to %s at %s", url, mountPath)
	return storage.NewMountDataSource(op, f, cleanFunc), nil
}
