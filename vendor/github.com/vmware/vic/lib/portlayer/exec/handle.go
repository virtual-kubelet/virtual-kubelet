// Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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

package exec

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"context"

	log "github.com/Sirupsen/logrus"
	"github.com/golang/groupcache/lru"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/guest"
	"github.com/vmware/vic/lib/portlayer/util"
	"github.com/vmware/vic/lib/spec"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/extraconfig/vmomi"
	"github.com/vmware/vic/pkg/vsphere/session"
)

// Resources describes the resource allocation for the containerVM
type Resources struct {
	NumCPUs  int64
	MemoryMB int64
}

// ContainerCreateConfig defines the parameters for Create call
type ContainerCreateConfig struct {
	Metadata *executor.ExecutorConfig

	Resources Resources
}

var handles *lru.Cache
var handlesLock sync.Mutex

const (
	handleLen = 16
	lruSize   = 1000
)

func init() {
	handles = lru.New(lruSize)
}

type Handle struct {
	// copy from container cache
	containerBase

	// The guest used to generate specific device types
	Guest guest.Guest

	// desired spec
	Spec *spec.VirtualMachineConfigSpec
	// desired changes to extraconfig
	changes []types.BaseOptionValue

	// desired state
	targetState State

	// should this change trigger a reload in the target container
	reload bool

	// allow for passing outside of the process
	key string
}

func newHandleKey() string {
	b := make([]byte, handleLen)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic(err) // This shouldn't happen
	}
	return hex.EncodeToString(b)
}

// Added solely to support testing - need a better way to do this
func TestHandle(id string) *Handle {
	defer trace.End(trace.Begin("Handle.Create"))

	h := newHandle(&Container{})
	h.ExecConfig.ID = id

	return h
}

// newHandle creates a handle for an existing container
// con must not be nil
func newHandle(con *Container) *Handle {
	h := &Handle{
		key:           newHandleKey(),
		targetState:   StateUnknown,
		containerBase: *newBase(con.vm, con.Config, con.Runtime),
		// currently every operation has a spec, because even the power operations
		// make changes to extraconfig for timestamps and session status
		Spec: &spec.VirtualMachineConfigSpec{
			VirtualMachineConfigSpec: &types.VirtualMachineConfigSpec{},
		},
	}

	handlesLock.Lock()
	defer handlesLock.Unlock()

	handles.Add(h.key, h)

	return h
}

func (h *Handle) TargetState() State {
	return h.targetState
}

func (h *Handle) SetTargetState(s State) {
	h.targetState = s
}

func (h *Handle) Reload() {
	h.reload = true
}

// Rename updates the container name in ExecConfig as well as the vSphere display name
func (h *Handle) Rename(op trace.Operation, newName string) *Handle {
	defer trace.End(trace.Begin(newName))

	h.ExecConfig.Name = newName

	s := &spec.VirtualMachineConfigSpecConfig{
		ID:   h.ExecConfig.ID,
		Name: newName,
	}

	h.Spec.Spec().Name = util.DisplayName(op, s, Config.ContainerNameConvention)

	return h
}

// GetHandle finds and returns the handle that is referred by key
func GetHandle(key string) *Handle {
	handlesLock.Lock()
	defer handlesLock.Unlock()

	if h, ok := handles.Get(key); ok {
		return h.(*Handle)
	}

	return nil
}

// HandleFromInterface returns the Handle
func HandleFromInterface(key interface{}) *Handle {
	defer trace.End(trace.Begin(""))

	if h, ok := key.(string); ok {
		return GetHandle(h)
	}

	log.Errorf("Type assertion failed for %#+v", key)
	return nil
}

// ReferenceFromHandle returns the reference of the given handle
func ReferenceFromHandle(handle interface{}) interface{} {
	defer trace.End(trace.Begin(""))

	if h, ok := handle.(*Handle); ok {
		return h.String()
	}

	log.Errorf("Type assertion failed for %#+v", handle)
	return nil
}

func (h *Handle) String() string {
	return h.key
}

func (h *Handle) Commit(op trace.Operation, sess *session.Session, waitTime *int32) error {

	cfg := make(map[string]string)

	// Set timestamps based on target state
	switch h.TargetState() {
	case StateRunning:
		for _, sc := range h.ExecConfig.Sessions {
			sc.StartTime = time.Now().UTC().Unix()
			sc.Started = ""
			sc.ExitStatus = 0
		}
	case StateStopped:
		for _, sc := range h.ExecConfig.Sessions {
			sc.StopTime = time.Now().UTC().Unix()
			sc.Started = ""
		}
	}

	s := h.Spec.Spec()
	if h.Config != nil {
		s.ChangeVersion = h.Config.ChangeVersion
	}

	// if runtime is nil, should be fresh container create
	var filter int
	if h.Runtime == nil || h.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOff || h.TargetState() == StateStopped {
		// any values set with VM powered off are inherently persistent
		filter = ^extraconfig.NonPersistent
	} else {
		filter = extraconfig.NonPersistent | extraconfig.Hidden
	}

	extraconfig.Encode(extraconfig.ScopeFilterSink(uint(filter), extraconfig.MapSink(cfg)), h.ExecConfig)

	// strip unmodified keys from the update
	if h.Config != nil {
		h.changes = append(s.ExtraConfig, vmomi.OptionValueUpdatesFromMap(h.Config.ExtraConfig, cfg)...)
	} else {
		h.changes = append(s.ExtraConfig, vmomi.OptionValueFromMap(cfg, true)...)
	}
	s.ExtraConfig = h.changes

	if err := Commit(op, sess, h, waitTime); err != nil {
		return err
	}

	h.Close()
	return nil
}

// refresh is for internal use only - it's sole purpose at this time is to allow the stop path to update ChangeVersion
// and corresponding state before performing any associated reconfigure
func (h *Handle) refresh(op trace.Operation) {
	// update Config and Runtime to reflect current state
	h.containerBase.refresh(op)

	// reapply extraconfig changes
	s := h.Spec.Spec()

	s.ExtraConfig = h.changes
	s.ChangeVersion = h.Config.ChangeVersion
}

func (h *Handle) Close() {
	handlesLock.Lock()
	defer handlesLock.Unlock()

	handles.Remove(h.key)
}

// Create returns a new handle that can be Committed to create a new container.
// At this time the config is *not* deep copied so should not be changed once passed
//
// TODO: either deep copy the configuration, or provide an alternative means of passing the data that
// avoids the need for the caller to unpack/repack the parameters
func Create(ctx context.Context, vmomiSession *session.Session, config *ContainerCreateConfig) (*Handle, error) {
	op := trace.FromContext(ctx, "Handle.Create")
	defer trace.End(trace.Begin(config.Metadata.Name, op))

	h := &Handle{
		key:         newHandleKey(),
		targetState: StateCreated,
		containerBase: containerBase{
			ExecConfig: config.Metadata,
		},
	}

	// configure with debug
	h.ExecConfig.Diagnostics.DebugLevel = Config.DebugLevel
	h.ExecConfig.Diagnostics.SysLogConfig = Config.SysLogConfig

	// Convert the management hostname to IP
	ips, err := net.LookupIP(constants.ManagementHostName)
	if err != nil {
		log.Errorf("Unable to look up %s during create of %s: %s", constants.ManagementHostName, config.Metadata.ID, err)
		return nil, err
	}

	if len(ips) == 0 {
		log.Errorf("No IP found for %s during create of %s", constants.ManagementHostName, config.Metadata.ID)
		return nil, fmt.Errorf("No IP found on %s", constants.ManagementHostName)
	}

	if len(ips) > 1 {
		log.Errorf("Multiple IPs found for %s during create of %s: %v", constants.ManagementHostName, config.Metadata.ID, ips)
		return nil, fmt.Errorf("Multiple IPs found on %s: %#v", constants.ManagementHostName, ips)
	}

	uuid, err := instanceUUID(config.Metadata.ID)
	if err != nil {
		detail := fmt.Sprintf("unable to get instance UUID: %s", err)
		log.Error(detail)
		return nil, errors.New(detail)
	}

	specconfig := &spec.VirtualMachineConfigSpecConfig{
		NumCPUs:  int32(config.Resources.NumCPUs),
		MemoryMB: config.Resources.MemoryMB,

		ID:       config.Metadata.ID,
		Name:     config.Metadata.Name,
		BiosUUID: uuid,

		// TODO: make this toggle for pod or single based on number of images joined
		BootMediaPath: Config.BootstrapImagePath,
		VMPathName:    fmt.Sprintf("[%s]", vmomiSession.Datastore.Name()),

		Metadata: config.Metadata,
	}

	// if not vsan, set the datastore folder name to containerID
	if !vmomiSession.IsVSAN(op) {
		specconfig.VMPathName = fmt.Sprintf("[%s] %s/%s.vmx", vmomiSession.Datastore.Name(), specconfig.ID, specconfig.ID)
	}

	specconfig.VMFullName = util.DisplayName(op, specconfig, Config.ContainerNameConvention)

	// log only core portions
	s := specconfig
	log.Debugf("id: %s, name: %s, cpu: %d, mem: %d, parent: %s, os: %s, path: %s", s.ID, s.Name, s.NumCPUs, s.MemoryMB, s.ParentImageID, s.BootMediaPath, s.VMPathName)
	m := s.Metadata
	log.Debugf("annotations: %#v, reponame: %s", m.Annotations, m.RepoName)
	for name, sess := range m.Sessions {
		log.Debugf("session: %s, path: %s, dir: %s, runblock: %t, tty: %t, restart: %t, stdin: %t, stopsig: %s",
			name, sess.Cmd.Path, sess.Cmd.Dir, sess.RunBlock, sess.Tty, sess.Restart, sess.OpenStdin, sess.StopSignal)
	}

	// If the debug level is high, dump everything
	// we still do the logging above for consistency so searching the logs for common strings works.
	// TODO: move this into a debug level aware structure renderer
	if Config.DebugLevel > 2 {
		log.Debugf("Config: %#v", specconfig)
		log.Debugf("Executor spec: %#v", *specconfig.Metadata)
		for _, sess := range m.Sessions {
			log.Debugf("Session spec: %#v", *sess)
		}
	}

	// Create a linux guest
	linux, err := guest.NewLinuxGuest(op, vmomiSession, specconfig)
	if err != nil {
		log.Errorf("Failed during linux specific spec generation during create of %s: %s", config.Metadata.ID, err)
		return nil, err
	}

	h.Guest = linux
	h.Spec = linux.Spec()

	handlesLock.Lock()
	defer handlesLock.Unlock()

	handles.Add(h.key, h)

	return h, nil
}
