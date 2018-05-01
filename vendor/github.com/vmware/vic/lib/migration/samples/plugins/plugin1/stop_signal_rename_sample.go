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
	"github.com/vmware/vic/lib/migration/manager"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/session"
)

// sample plugin to migrate data in appliance configuration VirtualContainerHost
// If only a couple of items changed in the configuration, you don't have to copy all VirtualContainerHost. Only define the few items used by
// this upgrade plugin will simplify the extraconfig encoding/decoding process
const (
	version = 1
	target  = manager.ApplianceConfigure
)

func init() {
	defer trace.End(trace.Begin(fmt.Sprintf("Registering plugin %s:%d", target, version)))
	if err := manager.Migrator.Register(version, target, &ApplianceStopSignalRename{}); err != nil {
		log.Errorf("Failed to register plugin %s:%d, %s", target, version, err)
	}
}

// ApplianceStopSignalRename is plugin for vic 0.8.0-GA version upgrade
type ApplianceStopSignalRename struct {
}

type OldStopSignal struct {
	ExecutorConfig `vic:"0.1" scope:"read-only" key:"init"`
}

type ExecutorConfig struct {
	Sessions map[string]*SessionConfig `vic:"0.1" scope:"read-only" key:"sessions"`
}

type SessionConfig struct {
	StopSignal string `vic:"0.1" scope:"read-only" key:"stopSignal"`
}

type NewStopSignal struct {
	NewExecutorConfig `vic:"0.1" scope:"read-only" key:"init"`
}

type NewExecutorConfig struct {
	Sessions map[string]*NewSessionConfig `vic:"0.1" scope:"read-only" key:"sessions"`
}

type NewSessionConfig struct {
	StopSignal string `vic:"0.1" scope:"read-only" key:"forceStopSignal"`
}

func (p *ApplianceStopSignalRename) Migrate(ctx context.Context, s *session.Session, data interface{}) error {
	defer trace.End(trace.Begin(fmt.Sprintf("%d", version)))
	if data == nil {
		return nil
	}
	mapData := data.(map[string]string)
	oldStruct := &OldStopSignal{}
	result := extraconfig.Decode(extraconfig.MapSource(mapData), oldStruct)
	if result == nil {
		return &errors.DecodeError{}
	}
	keys := extraconfig.CalculateKeys(oldStruct, "ExecutorConfig.Sessions.*.StopSignal", "")
	for _, key := range keys {
		log.Debugf("old %s:%s", key, mapData[key])
	}

	newStruct := &NewStopSignal{}
	if len(oldStruct.ExecutorConfig.Sessions) == 0 {
		return nil
	}
	newStruct.Sessions = make(map[string]*NewSessionConfig)
	for id, sess := range oldStruct.ExecutorConfig.Sessions {
		newSess := &NewSessionConfig{}
		newSess.StopSignal = sess.StopSignal
		newStruct.Sessions[id] = newSess
	}

	cfg := make(map[string]string)
	extraconfig.Encode(extraconfig.MapSink(cfg), newStruct)
	// remove old data
	for _, key := range keys {
		delete(mapData, key)
	}
	for k, v := range cfg {
		log.Debugf("New data: %s:%s", k, v)
		mapData[k] = v
	}
	return nil
}
