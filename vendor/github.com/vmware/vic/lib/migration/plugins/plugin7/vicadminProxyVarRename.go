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

package plugin7

import (
	"context"
	"fmt"
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
	target      = manager.ApplianceConfigure
	oldHProxy   = "HTTP_PROXY"
	newHProxy   = "VICADMIN_HTTP_PROXY"
	oldSProxy   = "HTTPS_PROXY"
	newSProxy   = "VICADMIN_HTTPS_PROXY"
	sessionName = "vicadmin"
)

func init() {
	defer trace.End(trace.Begin(fmt.Sprintf("Registering plugin %s:%d", target, feature.VicadminProxyVarRenameVersion)))
	if err := manager.Migrator.Register(feature.VicadminProxyVarRenameVersion, target, &VicadminProxyVarRename{}); err != nil {
		log.Errorf("Failed to register plugin %s:%d, %s", target, feature.VicadminProxyVarRenameVersion, err)
	}
}

// VicadminProxyVarRename is plugin for vic 0.8.0-GA version upgrade
type VicadminProxyVarRename struct {
}

type VCHConfig struct {
	ExecutorConfig `vic:"0.1" scope:"read-only" key:"init"`
}

type ExecutorConfig struct {
	Sessions map[string]*SessionConfig `vic:"0.1" scope:"read-only" key:"sessions"`
}

type SessionConfig struct {
	Cmd Cmd `vic:"0.1" scope:"read-only" key:"cmd"`
}

type Cmd struct {
	Env []string `vic:"0.1" scope:"read-only" key:"Env"`
}

func (p *VicadminProxyVarRename) Migrate(ctx context.Context, s *session.Session, data interface{}) error {
	defer trace.End(trace.Begin(fmt.Sprintf("%d", feature.VicadminProxyVarRenameVersion)))
	if data == nil {
		return nil
	}
	mapData := data.(map[string]string)
	oldStruct := &VCHConfig{}
	result := extraconfig.Decode(extraconfig.MapSource(mapData), oldStruct)
	if result == nil {
		return &errors.DecodeError{}
	}

	// translate old proxy env var keys into to proxy env var keys
	// skip upgrading if the proxy isn't defined or something is wrong with the the vicadmin executor
	if oldStruct.Sessions == nil || oldStruct.Sessions[sessionName] == nil || oldStruct.Sessions[sessionName].Cmd.Env == nil {
		log.Debugln("vicadmin session not found. skipping proxy rename")
	} else {
		var newEnvs []string
		for _, envVar := range oldStruct.Sessions[sessionName].Cmd.Env {
			envVarArgs := strings.Split(envVar, "=")
			envVarValue := ""
			if len(envVarArgs) > 1 {
				envVarValue = envVarArgs[1]
			}
			if strings.Contains(envVar, oldHProxy) {
				newEnvs = append(newEnvs, fmt.Sprintf("%s=%s", newHProxy, envVarValue))
			} else if strings.Contains(envVar, oldSProxy) {
				newEnvs = append(newEnvs, fmt.Sprintf("%s=%s", newSProxy, envVarValue))
			} else {
				newEnvs = append(newEnvs, envVar)
			}
		}
		oldStruct.Sessions[sessionName].Cmd.Env = newEnvs
	}

	cfg := make(map[string]string)
	extraconfig.Encode(extraconfig.MapSink(cfg), oldStruct)
	for k, v := range cfg {
		log.Debugf("New data: %s:%s", k, v)
		mapData[k] = v
	}
	return nil
}
