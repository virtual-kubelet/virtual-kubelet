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

package manager

import (
	"context"
	"fmt"
	"sort"
	"sync"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/migration/errors"
	"github.com/vmware/vic/lib/migration/feature"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/session"
)

const (
	ApplianceConfigure = "ApplianceConfigure"
	ContainerConfigure = "ContainerConfigure"

	ApplianceVersionKey = "guestinfo.vice./init/version/PluginVersion"
	ContainerVersionKey = "guestinfo.vice./version/PluginVersion"
)

var (
	Migrator = NewDataMigrator()
)

type Plugin interface {
	Migrate(ctx context.Context, s *session.Session, data interface{}) error
}

type DataMigration interface {
	// Register plugin to data migration system
	Register(version int, target string, plugin Plugin) error
	// Migrate data with current version ID, return maximum ID number of executed plugins
	Migrate(ctx context.Context, s *session.Session, target string, currentVersion int, data interface{}) (int, error)
	// LatestVersion return the latest plugin version for specified target
	LatestVersion(target string) int
}

type DataMigrator struct {
	targetVers map[string][]int
	verPlugins map[int]Plugin
	once       sync.Once
}

func NewDataMigrator() DataMigration {
	return &DataMigrator{
		targetVers: make(map[string][]int),
		verPlugins: make(map[int]Plugin),
	}
}

// Register plugin to data migration system
func (m *DataMigrator) Register(ver int, target string, plugin Plugin) error {
	defer trace.End(trace.Begin(fmt.Sprintf("plugin %s:%d", target, ver)))
	// assert if plugin version less than max plugin version, which is forcing deveoper to change MaxPluginVersion variable everytime new plugin is added
	if plugin == nil {
		return &errors.InternalError{
			Message: "Empty Plugin object is not allowed",
		}
	}
	if ver > feature.MaxPluginVersion {
		return &errors.InternalError{
			Message: fmt.Sprintf("Plugin %d is bigger than Max Plugin Version %d", ver, feature.MaxPluginVersion),
		}
	}

	if m.verPlugins[ver] != nil {
		return &errors.InternalError{
			Message: fmt.Sprintf("Plugin %d is conflict with another plugin, please make sure the plugin Version is unique and ascending", ver),
		}
	}
	m.targetVers[target] = append(m.targetVers[target], ver)
	m.verPlugins[ver] = plugin
	return nil
}

func (m *DataMigrator) sortVersions() {
	m.once.Do(func() {
		sort.Ints(m.targetVers[ApplianceConfigure])
		sort.Ints(m.targetVers[ContainerConfigure])
	})
}

// Migrate data with current version ID, return true if has any plugin executed
func (m *DataMigrator) Migrate(ctx context.Context, s *session.Session, target string, currentVersion int, data interface{}) (int, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("migrate %s from %d", target, currentVersion)))
	m.sortVersions()

	pluginVers := m.targetVers[target]
	if len(pluginVers) == 0 {
		log.Debugf("No plugins registered for %s", target)
		return currentVersion, nil
	}

	i := sort.SearchInts(pluginVers, currentVersion)
	if i >= len(pluginVers) {
		log.Debugf("No plugins greater than %d", currentVersion)
		return currentVersion, nil
	}

	latestVer := currentVersion
	j := i
	if pluginVers[i] == currentVersion {
		j = i + 1
	}
	for ; j < len(pluginVers); j++ {
		ver := pluginVers[j]
		p := m.verPlugins[ver]
		err := p.Migrate(ctx, s, data)
		if err != nil {
			return latestVer, err
		}
		latestVer = ver
	}
	return latestVer, nil
}

// LatestVersion return the latest plugin version for specified target
func (m *DataMigrator) LatestVersion(target string) int {
	pluginVers := m.targetVers[target]
	l := len(pluginVers)
	if l == 0 {
		log.Debugf("No plugin registered for %s", target)
		return 0
	}
	return pluginVers[l-1]
}
