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
	"fmt"
	"net/url"
)

// PortLayerVsphere is a WIP implementation of the execution.go interfaces
type PortLayerVsphere struct {
	vmomiGateway VmomiGateway
	handles      HandleFactory
	containers   map[ID]*container
}

func (p *PortLayerVsphere) getContainer(handle Handle) *container {
	return p.containers[handle.(*PendingCommit).ContainerID]
}

func (p *PortLayerVsphere) newHandle(cid ID) *PendingCommit {
	return p.handles.createHandle(cid).(*PendingCommit)
}

func (p *PortLayerVsphere) Init(gateway VmomiGateway, factory HandleFactory) error {
	p.handles = factory
	p.vmomiGateway = gateway
	p.containers = make(map[ID]*container)
	return nil
}

func (p *PortLayerVsphere) CreateContainer(name string) (Handle, error) {
	cid := GenerateID()
	handle := p.newHandle(cid)
	handle.config.Name = name
	handle.runState = Created
	return handle, nil
}

func (p *PortLayerVsphere) GetHandle(cid ID) (Handle, error) {
	c := p.containers[cid]
	if c == nil {
		return nil, fmt.Errorf("Invalid container ID")
	}
	return p.handles.createHandle(c.ContainerID), nil
}

func (p *PortLayerVsphere) SetEntryPoint(handle Handle, workDir string, execPath string, execArgs string) (Handle, error) {
	resolvedHandle := handle.(*PendingCommit)
	resolvedHandle.mainProcess = NewProcessConfig(workDir, execPath, execArgs)
	return p.handles.refreshHandle(handle), nil
}

func (p *PortLayerVsphere) Commit(handle Handle) (ID, error) {
	var err error
	c := p.getContainer(handle)
	if c == nil {
		c, err = p.createContainer(handle)
	} else {
		//		if c.vm == nil {
		//			return "", fmt.Errorf("Cannot modify container with no VM")
		//		}
		err = p.modifyContainer(c.runState, handle)
	}
	// Handle will be garbage collected
	return c.ContainerID, err
}

func (p *PortLayerVsphere) CopyTo(handle Handle, targetDir string, fname string, perms int16, data []byte) (Handle, error) {
	var result Handle
	resolvedHandle := handle.(*PendingCommit)
	u, err := url.Parse("file://" + targetDir + "/" + fname)
	if err == nil {
		fileToCopy := FileToCopy{target: *u, perms: perms, data: data}
		resolvedHandle.filesToCopy = append(resolvedHandle.filesToCopy, fileToCopy)
		result = p.handles.refreshHandle(handle)
	}
	return result, err
}

func (p *PortLayerVsphere) SetLimits(handle Handle, memoryMb int, cpuMhz int) (Handle, error) {
	resolvedHandle := handle.(*PendingCommit)
	resolvedHandle.config.Limits = ResourceLimits{MemoryMb: memoryMb, CPUMhz: cpuMhz}
	return p.handles.refreshHandle(handle), nil
}

func (p *PortLayerVsphere) SetRunState(handle Handle, runState RunState) (Handle, error) {
	resolvedHandle := handle.(*PendingCommit)
	resolvedHandle.runState = runState
	return p.handles.refreshHandle(handle), nil
}

func (p *PortLayerVsphere) DestroyContainer(cid ID) error {
	c := p.containers[cid]
	if c == nil {
		return fmt.Errorf("Invalid container ID")
	}
	delete(p.containers, cid)
	return nil
}

func (p *PortLayerVsphere) createContainer(handle Handle) (*container, error) {
	resolvedHandle := handle.(*PendingCommit)
	c := container{}
	p.containers[resolvedHandle.ContainerID] = &c
	c.ContainerID = resolvedHandle.ContainerID
	c.runState = resolvedHandle.runState
	// followed by other transfer of state from pending to container
	//	fmt.Printf("Creating container for %v\n", pending)
	return &c, nil
}

func (p *PortLayerVsphere) modifyContainer(runState RunState, handle Handle) error {
	// fmt.Printf("Modifying container for %v\n", pending)
	return nil
}
