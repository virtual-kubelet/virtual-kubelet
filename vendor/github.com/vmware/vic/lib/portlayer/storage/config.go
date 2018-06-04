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
	"github.com/vmware/govmomi/view"
	"github.com/vmware/vic/lib/config"
)

var Config Configuration

// Configuration is a slice of the VCH config that is relevant to the storage part of the port layer
type Configuration struct {
	// Turn on debug logging
	DebugLevel int `vic:"0.1" scope:"read-only" key:"init/diagnostics/debug"`

	// Port Layer - storage
	config.Storage `vic:"0.1" scope:"read-only" key:"storage"`

	// ContainerView
	// https://pubs.vmware.com/vsphere-6-0/index.jsp#com.vmware.wssdk.apiref.doc/vim.view.ContainerView.html
	ContainerView *view.ContainerView
}
