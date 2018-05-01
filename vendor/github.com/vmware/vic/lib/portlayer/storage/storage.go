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

package storage

import (
	"context"
	"io"
	"net/url"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

var (
	once sync.Once

	importers map[string]Importer
	exporters map[string]Exporter
)

type FileStat struct {
	LinkTarget string
	Mode       uint32
	Name       string
	Size       int64
	ModTime    time.Time
}

func init() {
	importers = make(map[string]Importer)
	exporters = make(map[string]Exporter)
}

func create(ctx context.Context, session *session.Session, pool *object.ResourcePool) error {
	var err error

	mngr := view.NewManager(session.Vim25())

	// Create view of VirtualMachine objects under the VCH's resource pool
	Config.ContainerView, err = mngr.CreateContainerView(ctx, pool.Reference(), []string{"VirtualMachine"}, true)
	if err != nil {
		return err
	}
	return nil
}

// Init performs basic initialization, including population of storage.Config
func Init(ctx context.Context, session *session.Session, pool *object.ResourcePool, source extraconfig.DataSource, _ extraconfig.DataSink) error {
	defer trace.End(trace.Begin(""))

	var err error

	once.Do(func() {
		// Grab the storage layer config blobs from extra config
		extraconfig.Decode(source, &Config)
		log.Debugf("Decoded VCH config for storage: %#v", Config)

		err = create(ctx, session, pool)
	})
	return err
}

// RegisterImporter registers the specified importer against the provided store for later retrieval.
func RegisterImporter(op trace.Operation, store string, i Importer) {
	op.Infof("Registering importer: %s => %T", store, i)

	importers[store] = i
}

// RegisterExporter registers the specified exporter against the provided store for later retrieval.
func RegisterExporter(op trace.Operation, store string, e Exporter) {
	op.Infof("Registering exporter: %s => %T", store, e)

	exporters[store] = e
}

// GetImporter retrieves an importer registered with the provided store.
// Will return nil, false if the store is not found.
func GetImporter(store string) (Importer, bool) {
	i, ok := importers[store]
	return i, ok
}

// GetExporter retrieves an exporter registered with the provided store.
// Will return nil, false if the store is not found.
func GetExporter(store string) (Exporter, bool) {
	e, ok := exporters[store]
	return e, ok
}

// GetImporters returns the set of known importers.
func GetImporters() []string {
	keys := make([]string, 0, len(importers))
	for key := range importers {
		keys = append(keys, key)
	}

	return keys
}

// GetExporters returns the set of known importers.
func GetExporters() []string {
	keys := make([]string, 0, len(exporters))
	for key := range exporters {
		keys = append(keys, key)
	}

	return keys
}

// Resolver defines methods for mapping ids to URLS, and urls to owners of that device
type Resolver interface {
	// URL returns a url to the data source representing `id`
	// For historic reasons this is not the same URL that other parts of the storage component use, but an actual
	// URL suited for locating the storage element without having additional precursor knowledge.
	URL(op trace.Operation, id string) (*url.URL, error)

	// Owners returns a list of VMs that are using the resource specified by `url`
	Owners(op trace.Operation, url *url.URL, filter func(vm *mo.VirtualMachine) bool) ([]*vm.VirtualMachine, error)
}

// DataSource defines the methods for exporting data from a specific storage element as a tar stream
type DataSource interface {
	// Close releases all resources associated with this source. Shared resources should be reference counted.
	io.Closer

	// Export performs an export of the specified files, returning the data as a tar stream. This is single use; once
	// the export has completed it should not be assumed that the source remains functional.
	//
	// spec: specifies which files will be included/excluded in the export and allows for path rebasing/stripping
	// data: if true the actual file data is included, if false only the file headers are present
	Export(op trace.Operation, spec *archive.FilterSpec, data bool) (io.ReadCloser, error)

	// Source returns the mechanism by which the data source is accessed
	// Examples:
	//     vmdk mounted locally: *os.File
	//     nfs volume:  		 XDR-client
	//     via guesttools:  	 toolbox client
	Source() interface{}

	// Stat stats the filesystem target indicated by the last entry in the given Filterspecs inclusion map
	Stat(op trace.Operation, spec *archive.FilterSpec) (*FileStat, error)
}

// DataSink defines the methods for importing data to a specific storage element from a tar stream
type DataSink interface {
	// Close releases all resources associated with this sink. Shared resources should be reference counted.
	io.Closer

	// Import performs an import of the tar stream to the source held by this DataSink.  This is single use; once
	// the export has completed it should not be assumed that the sink remains functional.
	//
	// spec: specifies which files will be included/excluded in the import and allows for path rebasing/stripping
	// tarStream: the tar stream to from which to import data
	Import(op trace.Operation, spec *archive.FilterSpec, tarStream io.ReadCloser) error

	// Sink returns the mechanism by which the data sink is accessed
	// Examples:
	//     vmdk mounted locally: *os.File
	//     nfs volume:  		 XDR-client
	//     via guesttools:  	 toolbox client
	Sink() interface{}
}

// Importer defines the methods needed to write data into a storage element. This should be implemented by the various
// store types.
type Importer interface {
	// Import allows direct construction and invocation of a data sink for the specified ID.
	Import(op trace.Operation, id string, spec *archive.FilterSpec, tarStream io.ReadCloser) error

	// NewDataSink constructs a data sink for the specified ID within the context of the Importer. This is a single
	// use sink which may hold resources until Closed.
	NewDataSink(op trace.Operation, id string) (DataSink, error)
}

// Exporter defines the methods needed to read data from a storage element, optionally diff with an ancestor. This
// shoiuld be implemented by the various store types.
type Exporter interface {
	// Export allows direct construction and invocation of a data source for the specified ID.
	Export(op trace.Operation, id, ancestor string, spec *archive.FilterSpec, data bool) (io.ReadCloser, error)

	// NewDataSource constructs a data source for the specified ID within the context of the Exporter. This is a single
	// use source which may hold resources until Closed.
	NewDataSource(op trace.Operation, id string) (DataSource, error)
}
