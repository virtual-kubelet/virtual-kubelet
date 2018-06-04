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

package tether

import (
	"testing"

	"github.com/docker/docker/pkg/stringid"

	"github.com/vmware/vic/lib/config/executor"
)

func TestSetHostname(t *testing.T) {
	_, mocker := testSetup(t)
	defer testTeardown(t, mocker)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "sethostname",
			Name: "tether_test_executor",
		},
	}

	tthr, _, _ := StartTether(t, &cfg, mocker)

	<-mocker.Started

	// prevent indefinite wait in tether - normally session exit would trigger this
	tthr.Stop()

	// wait for tether to exit
	<-mocker.Cleaned

	expected := stringid.TruncateID(cfg.ID)
	if mocker.Hostname != expected {
		t.Errorf("expected: %s, actual: %s", expected, mocker.Hostname)
	}
}

func TestNoNetwork(t *testing.T) {
	_, mocker := testSetup(t)
	defer testTeardown(t, mocker)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "ipconfig",
			Name: "tether_test_executor",
		},
	}

	tthr, _, _ := StartTether(t, &cfg, mocker)

	<-mocker.Started

	// prevent indefinite wait in tether - normally session exit would trigger this
	tthr.Stop()

	// wait for tether to exit
	<-mocker.Cleaned
}
