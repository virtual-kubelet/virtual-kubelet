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

package container

import (
	"errors"
	"net/url"
	"strings"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/disk"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// ContainerStorer defines the interface contract expected to allow import and export
// against containers
type ContainerStorer interface {
	storage.Resolver
	storage.Importer
	storage.Exporter
}

// ContainerStore stores container storage information
type ContainerStore struct {
	disk.Vmdk

	// used to resolve images when diffing
	images storage.Resolver
}

// NewContainerStore creates and returns a new container store
func NewContainerStore(op trace.Operation, s *session.Session, imageResolver storage.Resolver) (*ContainerStore, error) {
	dm, err := disk.NewDiskManager(op, s, storage.Config.ContainerView)
	if err != nil {
		return nil, err
	}

	cs := &ContainerStore{
		Vmdk: disk.Vmdk{
			Manager: dm,
			//ds: ds,
			Session: s,
		},

		images: imageResolver,
	}
	return cs, nil
}

// URL converts the id of a resource to a URL
func (c *ContainerStore) URL(op trace.Operation, id string) (*url.URL, error) {
	// using diskfinder with a basic suffix match is an inefficient and potentially error prone way of doing this
	// mapping, but until the container store has a structured means of knowing this information it's at least
	// not going to be incorrect without an ID collision.
	dsPath, err := c.DiskFinder(op, func(filename string) bool {
		return strings.HasSuffix(filename, id+".vmdk")
	})
	if err != nil {
		return nil, err
	}

	return &url.URL{
		Scheme: "ds",
		Path:   dsPath,
	}, nil
}

// Owners returns a list of VMs that are using the resource specified by `url`
func (c *ContainerStore) Owners(op trace.Operation, url *url.URL, filter func(vm *mo.VirtualMachine) bool) ([]*vm.VirtualMachine, error) {
	if url.Scheme != "ds" {
		return nil, errors.New("vmdk path must be a datastore url with \"ds\" scheme")
	}

	return c.Vmdk.Owners(op, url, filter)
}
