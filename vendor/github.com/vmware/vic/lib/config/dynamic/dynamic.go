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

package dynamic

import (
	"context"
	"errors"

	"github.com/vmware/vic/lib/config"
)

var ErrConfigNotModified = errors.New("config not modified")
var ErrAccessDenied = errors.New("access denied")
var ErrSourceUnavailable = errors.New("source not available")

// Source is configuration source, remote or otherwise
type Source interface {
	// Get returns a config object. If the remote/local source
	// is not available, Get returns nil, with an error indicating
	// what went wrong. If the configuration has not changed since
	// the last time Get was called, it returns
	// (nil, ErrConfigNotModified). If the remote/local source
	// denies access, it returns (nil, ErrAccessDenied).
	Get(ctx context.Context) (*config.VirtualContainerHostConfigSpec, error)
}

// Merger is used to merge two config objects
type Merger interface {
	// Merge merges two config objects returning the resulting config.
	Merge(orig, other *config.VirtualContainerHostConfigSpec) (*config.VirtualContainerHostConfigSpec, error)
}
