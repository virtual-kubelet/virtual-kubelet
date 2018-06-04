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

// Package plugin5 Plugin to migrate urls from go1.7 to go1.8
// Issue# https://github.com/vmware/vic/issues/4388
package plugin5

import (
	"context"
	"fmt"
	"net/url"
	"strings"

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

// VirtualContainerHostConfigSpec holds the metadata for a
// Virtual Container Host that should be visible inside the appliance VM.
type VirtualContainerHostConfigSpec struct {
	// Registry configuration for Imagec
	Registry `vic:"0.1" scope:"read-only" key:"registry"`
}

// Registry defines the registries virtual container host can talk to
type Registry struct {
	// Insecure registries
	InsecureRegistries []url.URL `vic:"0.1" scope:"read-only" key:"insecure_registries"`
}

func init() {
	defer trace.End(trace.Begin(fmt.Sprintf("Registering plugin %s:%d", target, feature.MigrateRegistryVersion)))
	if err := manager.Migrator.Register(feature.MigrateRegistryVersion, target, &MigrateRegistry{}); err != nil {
		log.Errorf("Failed to register plugin %s:%d, %s", target, feature.MigrateRegistryVersion, err)
		panic(err)
	}
}

// MigrateRegistry is plugin for vic 0.9.0-GA version upgrade
type MigrateRegistry struct {
}

func (p *MigrateRegistry) Migrate(ctx context.Context, s *session.Session, data interface{}) error {
	defer trace.End(trace.Begin(fmt.Sprintf("MigrateRegistry version %d", feature.MigrateRegistryVersion)))
	if data == nil {
		log.Debugf("No data received plugin %s:%d", target, feature.MigrateRegistryVersion)
		return nil
	}

	mapData, ok := data.(map[string]string)
	if !ok {
		// Log the error here and return nil so that other plugins can proceed
		log.Errorf("Migration data format is not map: %+v", data)
		return nil
	}

	oldVCHSpec := &VirtualContainerHostConfigSpec{}

	result := extraconfig.Decode(extraconfig.MapSource(mapData), oldVCHSpec)
	if result == nil {
		log.Errorf("Error decoding vchspec: %+v", oldVCHSpec)
		return &errors.DecodeError{}
	}

	log.Debugf("The oldVCHSpec is %+v", oldVCHSpec)

	newVCHSpec := &VirtualContainerHostConfigSpec{}

	for _, registry := range oldVCHSpec.InsecureRegistries {
		log.Debugf("Checking insecure registry url: %v", registry.String())
		if registry.Host == "" {
			log.Debugf("Fixing insecure registry url: %v", registry.String())

			// split host:port/path
			sp := strings.SplitN(registry.Path, "/", 2)

			// Fix host from the first index of split
			registry.Host = sp[0]
			// if a path was present, in the second index of split
			if len(sp) > 1 {
				registry.Path = sp[1]
			} else {
				// else set path as empty
				registry.Path = ""
			}
		}
		newVCHSpec.InsecureRegistries = append(newVCHSpec.InsecureRegistries, registry)
	}

	cfg := make(map[string]string)
	extraconfig.Encode(extraconfig.MapSink(cfg), newVCHSpec)

	for k, v := range cfg {
		log.Debugf("New data: %s:%s", k, v)
		mapData[k] = v
	}

	return nil
}
