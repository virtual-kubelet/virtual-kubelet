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

package exec2

import (
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/vmware/vic/pkg/vsphere/vm"
)

type ID uuid.UUID

func GenerateID() ID {
	return ID(uuid.New())
}

func ParseID(idStr string) (ID, error) {
	result, err := uuid.Parse(idStr)
	return ID(result), err
}

func (id ID) String() string {
	return uuid.UUID(id).String()
}

// Struct that represents the internal port-layer representation of a container
// All data in this struct must be data that is either immutable
// or can be relied upon without having to query either the container guest
// or the underlying infrastructure. Some of this state will be updated by events
type container struct {
	ConstantConfig

	vm       vm.VirtualMachine
	runState RunState

	config       Config
	mainProcess  ProcessConfig // container main process
	execdProcess []ProcessConfig

	filesToCopy []FileToCopy // cache if copy while stopped
}

// config that will be applied to a container on commit
// Needs to be public as it will be shared by net, storage etc
type PendingCommit struct {
	ConstantConfig

	runState    RunState
	config      Config
	mainProcess ProcessConfig
	filesToCopy []FileToCopy
}

// config state that cannot change for the lifetime of the container
type ConstantConfig struct {
	ContainerID ID
	Created     time.Time
}

// variable container configuration state
type Config struct {
	Name   string
	Limits ResourceLimits
}

// configuration state of a container process
type ProcessConfig struct {
	ProcessID ID
	WorkDir   string
	ExecPath  string
	ExecArgs  string
}

func NewProcessConfig(workDir string, execPath string, execArgs string) ProcessConfig {
	return ProcessConfig{ProcessID: GenerateID(), WorkDir: workDir, ExecArgs: execArgs, ExecPath: execPath}
}

type ProcessStatus int

const (
	_ ProcessStatus = iota
	Started
	Exited
)

// runtime status of a container process
type ProcessRunState struct {
	ProcessID  ID
	Status     ProcessStatus
	GuestPid   int
	ExitCode   int
	ExitMsg    string
	StartedAt  time.Time
	FinishedAt time.Time
}

type FileToCopy struct {
	target url.URL
	perms  int16
	data   []byte
}

type ResourceLimits struct {
	MemoryMb int
	CPUMhz   int
}
