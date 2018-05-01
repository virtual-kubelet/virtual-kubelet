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

package plugin1

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

const (
	target = manager.ApplianceConfigure
)

func init() {
	defer trace.End(trace.Begin(fmt.Sprintf("Registering plugin %s:%d", target, feature.AddCommonSpecForVCHVersion)))
	if err := manager.Migrator.Register(feature.AddCommonSpecForVCHVersion, target, &AddCommonSpecForVCH{}); err != nil {
		log.Errorf("Failed to register plugin %s:%d, %s", target, feature.AddCommonSpecForVCHVersion, err)
		panic(err)
	}
}

// AddCommonSpecForVCH is plugin for vic 0.8.0-GA version upgrade
type AddCommonSpecForVCH struct {
}

type VirtualContainerHostConfigSpec struct {
	ExecutorConfig `vic:"0.1" scope:"read-only" key:"init"`
}

type ExecutorConfig struct {
	Common `vic:"0.1" scope:"read-only" key:"common"`
}

type Common struct {
	// A reference to the components hosting execution environment, if any
	ExecutionEnvironment string

	// Unambiguous ID with meaning in the context of its hosting execution environment
	ID string `vic:"0.1" scope:"read-only" key:"id"`

	// Convenience field to record a human readable name
	Name string `vic:"0.1" scope:"read-only" key:"name"`

	// Freeform notes related to the entity
	Notes string `vic:"0.1" scope:"hidden" key:"notes"`
}

type UpdatedVCHConfigSpec struct {
	UpdatedExecutorConfig `vic:"0.1" scope:"read-only" key:"init"`
}

type UpdatedExecutorConfig struct {
	UpdatedCommon `vic:"0.1" scope:"read-only" key:"common"`
}

type UpdatedCommon struct {
	// A reference to the components hosting execution environment, if any
	ExecutionEnvironment string

	// Unambiguous ID with meaning in the context of its hosting execution environment
	ID string `vic:"0.1" scope:"read-only" key:"id"`

	// Convenience field to record a human readable name
	Name string `vic:"0.1" scope:"hidden" key:"name"`

	// Freeform notes related to the entity
	Notes string `vic:"0.1" scope:"hidden" key:"notes"`
}

func (p *AddCommonSpecForVCH) Migrate(ctx context.Context, s *session.Session, data interface{}) error {
	defer trace.End(trace.Begin(fmt.Sprintf("AddCommonSpecForVCH version: %d", feature.AddCommonSpecForVCHVersion)))
	if data == nil {
		return nil
	}
	mapData, ok := data.(map[string]string)
	if !ok {
		// Log the error here and return nil so that other plugins can proceed
		log.Errorf("Migration data format is not map: %+v", data)
		return nil
	}
	oldStruct := &VirtualContainerHostConfigSpec{}
	result := extraconfig.Decode(extraconfig.MapSource(mapData), oldStruct)
	log.Debugf("The oldStruct is %+v", oldStruct)
	if result == nil {
		return &errors.DecodeError{Err: fmt.Errorf("decode oldStruct %+v failed", oldStruct)}
	}

	newStruct := &UpdatedVCHConfigSpec{
		UpdatedExecutorConfig: UpdatedExecutorConfig{
			UpdatedCommon: UpdatedCommon{
				Name:                 oldStruct.Name,
				ID:                   oldStruct.ID,
				ExecutionEnvironment: oldStruct.ExecutionEnvironment,
				Notes:                oldStruct.Notes,
			},
		},
	}

	cfg := make(map[string]string)
	extraconfig.Encode(extraconfig.MapSink(cfg), newStruct)

	for k, v := range cfg {
		log.Debugf("New data: %s:%s", k, v)
		mapData[k] = v
	}
	return nil
}
