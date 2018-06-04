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

package plugin9

import (
	"context"
	"fmt"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/migration/errors"
	"github.com/vmware/vic/lib/migration/feature"
	"github.com/vmware/vic/lib/migration/manager"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/session"
)

const target = manager.ContainerConfigure

func init() {
	defer trace.End(trace.Begin(fmt.Sprintf("Registering plugin %s:%d", target, feature.ContainerCreateTimestampVersion)))
	if err := manager.Migrator.Register(feature.ContainerCreateTimestampVersion, target, &ContainerCreateTimestampVersion{}); err != nil {
		log.Errorf("Failed to register plugin %s:%d, %s", target, feature.ContainerCreateTimestampVersion, err)
		panic(err)
	}
}

// ContainerCreateTimestampVersion is a plugin to convert stored container create timestamps
// in seconds to nanoseconds in the container configuration.
type ContainerCreateTimestampVersion struct {
}

// ExecutorConfig is used to update the container create time from seconds to nanoseconds.
type ExecutorConfig struct {
	CreateTime int64 `vic:"0.1" scope:"read-write" key:"createtime"`
}

// Migrate converts the stored container create timestamp (in seconds) to nanoseconds.
func (c *ContainerCreateTimestampVersion) Migrate(ctx context.Context, s *session.Session, data interface{}) error {
	defer trace.End(trace.Begin(fmt.Sprintf("ContainerCreateTimestampVersion version %d", feature.ContainerCreateTimestampVersion)))
	if data == nil {
		return nil
	}
	mapData, ok := data.(map[string]string)
	if !ok {
		// Log the error here and return nil so that other plugins can proceed.
		log.Errorf("Migration data format is not map: %+v", data)
		return nil
	}
	oldStruct := &ExecutorConfig{}
	result := extraconfig.Decode(extraconfig.MapSource(mapData), oldStruct)
	log.Debugf("The oldStruct is %+v", oldStruct)
	if result == nil {
		return &errors.DecodeError{Err: fmt.Errorf("decode oldStruct %+v failed", oldStruct)}
	}

	// Convert create timestamp to nanoseconds for older containers that use seconds.
	oldStruct.CreateTime *= 1e9

	cfg := make(map[string]string)
	extraconfig.Encode(extraconfig.MapSink(cfg), oldStruct)

	for k, v := range cfg {
		log.Debugf("New data: %s:%s", k, v)
		mapData[k] = v
	}
	return nil
}
