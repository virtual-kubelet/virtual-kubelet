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

package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/pkg/trace"
)

// MountDataSource implements the DataSource interface for mounted devices
type MountDataSource struct {
	Path    *os.File
	Clean   func()
	cleanOp trace.Operation
	cancel  func()
	done    sync.WaitGroup
}

// NewMountDataSource creates a new data source associated with a specific mount, with the mount
// point being the path argument.
// The cleanup function is invoked with the Close of the ReadCloser from Export, or explicitly
func NewMountDataSource(op trace.Operation, path *os.File, cleanup func()) *MountDataSource {
	if path == nil {
		return nil
	}

	op.Debugf("Created mount data source at %s", path.Name())

	return &MountDataSource{
		Path:    path,
		Clean:   cleanup,
		cleanOp: trace.FromOperation(op, "clean up from new mount source"),
	}
}

// Source returns the data source associated with the DataSource
func (m *MountDataSource) Source() interface{} {
	return m.Path
}

// Export reads data from the associated data source and returns it as a tar archive
func (m *MountDataSource) Export(op trace.Operation, spec *archive.FilterSpec, data bool) (io.ReadCloser, error) {
	// reparent cleanup to Export operation
	m.cleanOp = trace.FromOperation(op, "clean up from export")
	notifyOp := trace.WithValue(&op, archive.CancelNotifyKey{}, &m.done, "with cancel notifier")
	cop, cancel := trace.WithCancel(&notifyOp, "cancellable export from mount")
	m.cancel = cancel

	name := m.Path.Name()
	fi, err := m.Path.Stat()
	if err != nil {
		op.Errorf("Unable to stat mount path %s for data source: %s", name, err)
		return nil, err
	}

	if !fi.IsDir() {
		return nil, errors.New("path must be a directory")
	}

	op.Infof("Exporting data from %s", name)
	// Diff is supplied "" to indicate that we are performing a read against a single target.
	rc, err := archive.Diff(cop, name, "", spec, data, false)

	// return the proxy regardless of error so that Close can be called
	return &ProxyReadCloser{
		rc,
		m.Close,
	}, err
}

// Stat stats the filesystem target indicated by the last entry in the given Filterspecs inclusion map
func (m *MountDataSource) Stat(op trace.Operation, spec *archive.FilterSpec) (*FileStat, error) {
	// retrieve relative path

	var targetPath string
	for path := range spec.Inclusions {
		targetPath = path
	}

	filePath := filepath.Join(m.Path.Name(), targetPath)
	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		// Does not exist is an expected result so no errors logged
		if !os.IsNotExist(err) {
			op.Errorf("failed to stat file: %s", err)
		}
		return nil, err
	}

	var linkTarget string
	// check for symlink
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		linkTarget, err = os.Readlink(filePath)
		if err != nil {
			return nil, err
		}
	}

	return &FileStat{linkTarget, uint32(fileInfo.Mode()), fileInfo.Name(), fileInfo.Size(), fileInfo.ModTime()}, nil
}

func (m *MountDataSource) Close() error {
	m.cleanOp.Infof("cleaning up after export - waiting for cancelation completion if necessary")

	// trigger cancelation of any ongoing operations
	if m.cancel != nil {
		m.cancel()
	}

	// wait for cancelation to take effect
	m.done.Wait()

	m.Path.Close()
	if m.Clean != nil {
		m.cleanOp.Debugf("calling specified cleaner function")
		m.Clean()
	}

	return nil
}
