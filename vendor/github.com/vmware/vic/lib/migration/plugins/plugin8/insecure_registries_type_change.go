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

package plugin8

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

// Translates the proxy environment variables from the old appliance to the new appliance for vic admin
const (
	target = manager.ApplianceConfigure
)

func init() {
	defer trace.End(trace.Begin(fmt.Sprintf("Registering plugin %s:%d", target, feature.InsecureRegistriesTypeChangeVersion)))
	if err := manager.Migrator.Register(feature.InsecureRegistriesTypeChangeVersion, target, &InsecureRegistriesTypeChange{}); err != nil {
		log.Errorf("Failed to register plugin %s:%d, %s", target, feature.InsecureRegistriesTypeChangeVersion, err)
	}
}

// InsecureRegistriesTypeChange
type InsecureRegistriesTypeChange struct {
}

type OldVCHConfig struct {
	OldRegistry `vic:"0.1" scope:"read-only" key:"registry"`
}

type OldRegistry struct {
	InsecureRegistries []url.URL `vic:"0.1" scope:"read-only" key:"insecure_registries"`
}

type VCHConfig struct {
	Registry `vic:"0.1" scope:"read-only" key:"registry"`
}

type Registry struct {
	InsecureRegistries []string `vic:"0.1" scope:"read-only" key:"insecure_registries"`
}

func (p *InsecureRegistriesTypeChange) Migrate(ctx context.Context, s *session.Session, data interface{}) error {
	defer trace.End(trace.Begin(fmt.Sprintf("%d", feature.InsecureRegistriesTypeChangeVersion)))

	if data == nil {
		return nil
	}
	mapData := data.(map[string]string)
	oldStruct := &OldVCHConfig{}
	result := extraconfig.Decode(extraconfig.MapSource(mapData), oldStruct)
	if result == nil {
		return &errors.DecodeError{}
	}

	vchConfig := &VCHConfig{}
	for _, r := range oldStruct.InsecureRegistries {
		vchConfig.InsecureRegistries = append(vchConfig.InsecureRegistries, strings.TrimPrefix(r.String(), r.Scheme+"://"))
	}

	cfg := make(map[string]string)
	extraconfig.Encode(extraconfig.MapSink(cfg), vchConfig)
	for k, v := range cfg {
		log.Debugf("New data: %s:%s", k, v)
		mapData[k] = v
	}
	return nil
}
