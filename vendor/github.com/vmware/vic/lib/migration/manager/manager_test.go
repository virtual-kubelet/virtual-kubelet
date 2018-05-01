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
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/session"
)

var testMap map[int]bool

type TestPlugin struct {
	Version int
}

func NewTestPlugin(version int) *TestPlugin {
	return &TestPlugin{version}
}

func (p *TestPlugin) Migrate(ctx context.Context, s *session.Session, data interface{}) error {
	testMap[p.Version] = true
	return nil
}

func setUp() {
	log.SetLevel(log.DebugLevel)
	trace.Logger.Level = log.DebugLevel
	testMap = make(map[int]bool)
}

func TestInsertID(t *testing.T) {
	setUp()

	tester := &DataMigrator{
		targetVers: make(map[string][]int),
		verPlugins: make(map[int]Plugin),
	}

	tester.targetVers[ApplianceConfigure] = []int{1, 11, 9, 5, 8, 2, 4}
	tester.sortVersions()
	assert.Equal(t, []int{1, 2, 4, 5, 8, 9, 11}, tester.targetVers[ApplianceConfigure], "Should have expected array")
	tester.targetVers[ApplianceConfigure] = append(tester.targetVers[ApplianceConfigure], []int{20, 15}...)

	// sort will only execute once
	tester.sortVersions()
	assert.NotEqual(t, []int{1, 2, 4, 5, 8, 9, 11, 15, 20}, tester.targetVers[ApplianceConfigure], "Should have expected array")
}

func TestMigratePluginExecution(t *testing.T) {
	setUp()

	tester := &DataMigrator{
		targetVers: make(map[string][]int),
		verPlugins: make(map[int]Plugin),
	}

	ids := []int{1, 2, 3, 4, 5}
	var err error
	for _, id := range ids {
		if err = tester.Register(id, ApplianceConfigure, NewTestPlugin(id)); err != nil {
			t.Errorf("Failed to register plugin %d: %s", id, err)
		}
	}

	dataID, err := tester.Migrate(nil, nil, ApplianceConfigure, 0, nil)
	assert.Equal(t, 5, dataID, "migrated id mismatch")
	for _, id := range ids {
		assert.True(t, testMap[id], fmt.Sprintf("plugin %d should be executed", id))
	}
	testMap = make(map[int]bool)
	dataID, err = tester.Migrate(nil, nil, ApplianceConfigure, 3, nil)
	assert.Equal(t, 5, dataID, "migrated id mismatch")
	for _, id := range ids[:3] {
		assert.False(t, testMap[id], fmt.Sprintf("plugin %d should not be executed", id))
	}
	for _, id := range ids[3:] {
		assert.True(t, testMap[id], fmt.Sprintf("plugin %d should be executed", id))
	}

	testMap = make(map[int]bool)
	dataID, err = tester.Migrate(nil, nil, ApplianceConfigure, 20, nil)
	assert.Equal(t, 20, dataID, "migrated id mismatch")
	for _, id := range ids {
		assert.False(t, testMap[id], fmt.Sprintf("plugin %d should not be executed", id))
	}
}
