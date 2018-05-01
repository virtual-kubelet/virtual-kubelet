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
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/iolog"
	"github.com/vmware/vic/lib/portlayer/event/events"
	stateevents "github.com/vmware/vic/lib/portlayer/event/events/vsphere"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/uid"
	"github.com/vmware/vic/pkg/vsphere/disk"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/sys"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/vm"

	log "github.com/Sirupsen/logrus"
	"github.com/google/uuid"
)

type State int

const (
	StateUnknown State = iota
	StateStarting
	StateRunning
	StateStopping
	StateStopped
	StateSuspending
	StateSuspended
	StateCreated
	StateCreating
	StateRemoving
	StateRemoved

	containerLogName = "output.log"

	vmNotSuspendedKey = "msg.suspend.powerOff.notsuspended"
	vmPoweringOffKey  = "msg.rpc.error.poweringoff"
)

func (s State) String() string {
	switch s {
	case StateCreated:
		return "Created"
	case StateStarting:
		return "Starting"
	case StateRunning:
		return "Running"
	case StateRemoving:
		return "Removing"
	case StateRemoved:
		return "Removed"
	case StateStopping:
		return "Stopping"
	case StateStopped:
		return "Stopped"
	case StateUnknown:
		return "Unknown"
	}
	return ""
}

// NotFoundError is returned when a types.ManagedObjectNotFound is returned from a vmomi call
type NotFoundError struct {
	err error
}

func (r NotFoundError) Error() string {
	return "VM has either been deleted or has not been fully created"
}

func IsNotFoundError(err error) bool {
	if soap.IsSoapFault(err) {
		fault := soap.ToSoapFault(err).VimFault()
		if _, ok := fault.(types.ManagedObjectNotFound); ok {
			return true
		}
	}
	return false
}

// RemovePowerError is returned when attempting to remove a containerVM that is powered on
type RemovePowerError struct {
	err error
}

func (r RemovePowerError) Error() string {
	return r.err.Error()
}

// ConcurrentAccessError is returned when concurrent calls tries to modify same object
type ConcurrentAccessError struct {
	err error
}

func (r ConcurrentAccessError) Error() string {
	return r.err.Error()
}

func IsConcurrentAccessError(err error) bool {
	_, ok := err.(ConcurrentAccessError)
	return ok
}

type DevicesInUseError struct {
	Devices []string
}

func (e DevicesInUseError) Error() string {
	return fmt.Sprintf("device %s in use", strings.Join(e.Devices, ","))
}

// Container is used to return data about a container during inspection calls
// It is a copy rather than a live reflection and does not require locking
type ContainerInfo struct {
	containerBase

	state State

	// Size of the leaf (unused)
	VMUnsharedDisk int64
}

// Container is used for an entry in the container cache - this is a "live" representation
// of containers in the infrastructure.
// DANGEROUS USAGE CONSTRAINTS:
//   None of the containerBase fields should be partially updated - consider them immutable once they're
//   part of a cache entry
//   i.e. Do not make changes in containerBase.ExecConfig - only swap, under lock, the pointer for a
//   completely new ExecConfig.
//   This constraint allows us to avoid deep copying those structs every time a container is inspected
type Container struct {
	m sync.Mutex

	ContainerInfo

	logFollowers []io.Closer

	newStateEvents map[State]chan struct{}
}

// newContainer constructs a Container suitable for adding to the cache
// it's state is set from the Runtime.PowerState field, or StateCreated if that is not
// viable
// This copies (shallow) the containerBase that's provided
func newContainer(base *containerBase) *Container {
	c := &Container{
		ContainerInfo: ContainerInfo{
			containerBase: *base,
			state:         StateCreated,
		},
		newStateEvents: make(map[State]chan struct{}),
	}

	// if this is a creation path, then Runtime will be nil
	if base.Runtime != nil {
		// set state
		switch base.Runtime.PowerState {
		case types.VirtualMachinePowerStatePoweredOn:
			// the containerVM is poweredOn, so set state to starting
			// then check to see if a start was successful
			c.state = StateStarting
			// If any sessions successfully started then set to running
			for _, s := range base.ExecConfig.Sessions {
				if s.Started != "" {
					c.state = StateRunning
					break
				}
			}
		case types.VirtualMachinePowerStatePoweredOff:
			// check if any of the sessions was started
			for _, s := range base.ExecConfig.Sessions {
				if s.Started != "" {
					c.state = StateStopped
					break
				}
			}
		case types.VirtualMachinePowerStateSuspended:
			c.state = StateSuspended
			log.Warnf("container VM %s: invalid power state %s", base.vm.Reference(), base.Runtime.PowerState)
		}
	}

	return c
}

func GetContainer(ctx context.Context, id uid.UID) *Handle {
	// get from the cache
	container := Containers.Container(id.String())
	if container != nil {
		return container.NewHandle(ctx)
	}

	return nil
}

func (c *ContainerInfo) String() string {
	return c.ExecConfig.ID
}

// State returns the state at the time the ContainerInfo object was created
func (c *ContainerInfo) State() State {
	return c.state
}

func (c *Container) String() string {
	return c.ExecConfig.ID
}

// Info returns a copy of the public container configuration that
// is consistent and copied under lock
func (c *Container) Info() *ContainerInfo {
	c.m.Lock()
	defer c.m.Unlock()

	info := c.ContainerInfo
	return &info
}

// CurrentState returns current state.
func (c *Container) CurrentState() State {
	c.m.Lock()
	defer c.m.Unlock()
	return c.state
}

// SetState changes container state.
func (c *Container) SetState(op trace.Operation, s State) State {
	c.m.Lock()
	defer c.m.Unlock()
	return c.updateState(op, s)
}

func (c *Container) updateState(op trace.Operation, s State) State {
	op.Debugf("Updating container %s state: %s->%s", c, c.state, s)
	prevState := c.state
	if s != c.state {
		c.state = s
		if ch, ok := c.newStateEvents[s]; ok {
			delete(c.newStateEvents, s)
			close(ch)
		}
	}
	return prevState
}

// transitionState changes the container state to finalState if the current state is initialState
// and returns an error otherwise.
func (c *Container) transitionState(op trace.Operation, initialState, finalState State) error {
	c.m.Lock()
	defer c.m.Unlock()

	if c.state == initialState {
		c.state = finalState
		op.Debugf("Set container %s state: %s->%s", c, initialState, finalState)
		return nil
	}

	return fmt.Errorf("container state is %s and was not changed to %s", c.state, finalState)
}

var closedEventChannel = func() <-chan struct{} {
	a := make(chan struct{})
	close(a)
	return a
}()

// WaitForState subscribes a caller to an event returning
// a channel that will be closed when an expected state is set.
// If expected state is already set the caller will receive a closed channel immediately.
func (c *Container) WaitForState(s State) <-chan struct{} {
	c.m.Lock()
	defer c.m.Unlock()

	if s == c.state {
		return closedEventChannel
	}

	if ch, ok := c.newStateEvents[s]; ok {
		return ch
	}

	eventChan := make(chan struct{})
	c.newStateEvents[s] = eventChan
	return eventChan
}

func (c *Container) NewHandle(ctx context.Context) *Handle {
	// Call property collector to fill the data
	if c.vm != nil {
		op := trace.FromContext(ctx, "NewHandle")
		// FIXME: this should be calling the cache to decide if a refresh is needed
		if err := c.Refresh(op); err != nil {
			op.Errorf("refreshing container %s failed: %s", c, err)
			return nil // nil indicates error
		}
	}

	// return a handle that represents zero changes over the current configuration
	// for this container
	return newHandle(c)
}

// Refresh updates config and runtime info, holding a lock only while swapping
// the new data for the old
func (c *Container) Refresh(op trace.Operation) error {
	c.m.Lock()
	defer c.m.Unlock()

	if err := c.refresh(op); err != nil {
		return err
	}

	// conditionally sync state (see issue 4872, 6372)
	event := stateevents.NewStateEvent(op, c.containerBase.Runtime.PowerState, c.VMReference())
	state := eventedState(op, event, c.state)

	// trigger internal event publishing if c.state -> state is a transition we care about
	// this will update container state and trigger follow up port layer events as needed
	c.onEvent(op, state, event)

	return nil
}

func (c *Container) refresh(op trace.Operation) error {
	return c.containerBase.refresh(op)
}

// RefreshFromHandle updates config and runtime info, holding a lock only while swapping
// the new data for the old
func (c *Container) RefreshFromHandle(op trace.Operation, h *Handle) {
	c.m.Lock()
	defer c.m.Unlock()

	if c.Config != nil && (h.Config == nil || h.Config.ChangeVersion != c.Config.ChangeVersion) {
		op.Warnf("container and handle ChangeVersions do not match for %s: %s != %s", c, c.Config.ChangeVersion, h.Config.ChangeVersion)
		return
	}

	// power off doesn't necessarily cause a change version increment and bug1898149 occasionally impacts power on
	if c.Runtime != nil && (h.Runtime == nil || h.Runtime.PowerState != c.Runtime.PowerState) {
		op.Warnf("container and handle PowerStates do not match: %s != %s", c.Runtime.PowerState, h.Runtime.PowerState)
		return
	}

	// copy over the new state
	c.containerBase = h.containerBase
	if c.Config != nil {
		op.Debugf("Update: updated change version from handle: %s", c.Config.ChangeVersion)
	}
}

// Start starts a container vm with the given params
func (c *Container) start(op trace.Operation) error {
	defer trace.End(trace.Begin(c.ExecConfig.ID, op))

	if c.vm == nil {
		return fmt.Errorf("vm not set")
	}
	// Set state to Starting
	c.SetState(op, StateStarting)

	err := c.containerBase.start(op)
	if err != nil {
		// change state to stopped because start task failed
		c.SetState(op, StateStopped)

		// check if locked disk error
		devices := disk.LockedDisks(err)
		if len(devices) > 0 {
			for i := range devices {
				// get device id from datastore file path
				// FIXME: find a reasonable way to get device ID from datastore path in exec
				devices[i] = strings.TrimSuffix(path.Base(devices[i]), ".vmdk")
			}
			return DevicesInUseError{devices}
		}
		return err
	}

	// wait task to set started field to something
	op, cancel := trace.WithTimeout(&op, constants.PropertyCollectorTimeout, "WaitForSession")
	defer cancel()

	err = c.waitForSession(op, c.ExecConfig.ID)
	if err != nil {
		// leave this in state starting - if it powers off then the event
		// will cause transition to StateStopped which is likely our original state
		// if the container was just taking a very long time it'll eventually
		// become responsive.

		// TODO: mechanism to trigger reinspection of long term transitional states
		return err
	}

	// Transition the state to Running only if it's Starting.
	// The current state is already Stopped if the container's process has exited or
	// a poweredoff event has been processed.
	if err = c.transitionState(op, StateStarting, StateRunning); err != nil {
		op.Debugf(err.Error())
	}

	return nil
}

func (c *Container) stop(op trace.Operation, waitTime *int32) error {
	defer trace.End(trace.Begin(c.ExecConfig.ID, op))

	defer c.onStop()

	// get existing state and set to stopping
	// if there's a failure we'll revert to existing
	finalState := c.SetState(op, StateStopping)

	err := c.containerBase.stop(op, waitTime)
	if err != nil {
		// we've got no idea what state the container is in at this point
		// running is an _optimistic_ statement
		// If the current state is Stopping, revert it to the old state.
		if stateErr := c.transitionState(op, StateStopping, finalState); stateErr != nil {
			op.Debugf(stateErr.Error())
		}

		return err
	}

	// Transition the state to Stopped only if it's Stopping.
	if err = c.transitionState(op, StateStopping, StateStopped); err != nil {
		op.Debugf(err.Error())
	}

	return nil
}

func (c *Container) Signal(op trace.Operation, num int64) error {
	defer trace.End(trace.Begin(c.ExecConfig.ID, op))

	if c.vm == nil {
		return fmt.Errorf("vm not set")
	}

	if num == int64(syscall.SIGKILL) {
		return c.containerBase.kill(op)
	}

	return c.startGuestProgram(op, "kill", fmt.Sprintf("%d", num))
}

func (c *Container) onStop() {
	lf := c.logFollowers
	c.logFollowers = nil

	log.Debugf("Container(%s) closing %d log followers", c, len(lf))
	for _, l := range lf {
		// #nosec: Errors unhandled.
		_ = l.Close()
	}
}

func (c *Container) LogReader(op trace.Operation, tail int, follow bool, since int64) (io.ReadCloser, error) {
	defer trace.End(trace.Begin(c.ExecConfig.ID, op))
	c.m.Lock()
	defer c.m.Unlock()

	if c.vm == nil {
		return nil, fmt.Errorf("vm not set")
	}

	url, err := c.vm.VMPathNameAsURL(op)
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("%s/%s", url.Path, containerLogName)

	var via string

	if c.state == StateRunning && c.vm.IsVC() {
		// #nosec: Errors unhandled.
		hosts, _ := c.vm.Datastore.AttachedHosts(op)
		if len(hosts) > 1 {
			// In this case, we need download from the VM host as it owns the file lock
			// #nosec: Errors unhandled.
			h, _ := c.vm.HostSystem(op)
			if h != nil {
				// get a context that embeds the host as a value
				ctx := c.vm.Datastore.HostContext(op, h)

				// revert the govmomi returned context to the previous op
				// the op was preserved as a value in the context
				op = trace.FromContext(ctx, "LogReader")
				via = fmt.Sprintf(" via %s", h.Reference())
			}
		}
	}

	op.Infof("pulling %s%s", name, via)

	file, err := c.vm.Datastore.Open(op, name)
	if err != nil {
		return nil, err
	}

	if since > 0 {
		err = file.TailFunc(tail, func(line int, message string) bool {
			if tail <= line && tail != -1 {
				return false
			}

			buf := bytes.NewBufferString(message)

			entry, err := iolog.ParseLogEntry(buf)
			if err != nil {
				op.Errorf("Error parsing log entry: %s", err.Error())
				return false
			}

			if entry.Timestamp.Unix() <= since {
				return false
			}

			return true
		})
	} else if tail >= 0 {
		err = file.Tail(tail)
		if err != nil {
			return nil, err
		}
	}

	if follow && c.state == StateRunning {
		follower := file.Follow(time.Second)

		c.logFollowers = append(c.logFollowers, follower)

		return follower, nil
	}

	return file, nil
}

// Remove removes a containerVM after detaching the disks
func (c *Container) Remove(op trace.Operation, sess *session.Session) error {
	// op := trace.FromContext(ctx, "Remove")
	defer trace.End(trace.Begin(c.ExecConfig.ID, op))
	c.m.Lock()
	defer c.m.Unlock()

	if c.vm == nil {
		return NotFoundError{}
	}

	// check state first
	if c.state == StateRunning {
		return RemovePowerError{fmt.Errorf("Container %s is powered on", c)}
	}

	// get existing state and set to removing
	// if there's a failure we'll revert to existing
	existingState := c.updateState(op, StateRemoving)

	// get the folder the VM is in
	url, err := c.vm.VMPathNameAsURL(op)
	if err != nil {

		// handle the out-of-band removal case
		if IsNotFoundError(err) {
			Containers.Remove(c.ExecConfig.ID)
			return NotFoundError{}
		}

		op.Errorf("Failed to get datastore path for %s: %s", c, err)
		c.updateState(op, existingState)
		return err
	}

	ds, err := sess.Finder.Datastore(op, url.Host)
	if err != nil {
		return err
	}

	// enable Destroy
	c.vm.EnableDestroy(op)

	concurrent := false
	// if DeleteExceptDisks succeeds on VC, it leaves the VM orphan so we need to call Unregister
	// if DeleteExceptDisks succeeds on ESXi, no further action needed
	// if DeleteExceptDisks fails, we should call Unregister and only return an error if that fails too
	//		Unregister sometimes can fail with ManagedObjectNotFound so we ignore it
	_, err = c.vm.WaitForResult(op, func(op context.Context) (tasks.Task, error) {
		return c.vm.DeleteExceptDisks(op)
	})
	if err != nil {
		f, ok := err.(types.HasFault)
		if !ok {
			op.Warnf("DeleteExceptDisks failed with non-fault error %s for %s.", err, c)

			c.updateState(op, existingState)
			return err
		}

		switch f.Fault().(type) {
		case *types.InvalidState:
			op.Warnf("container VM %s is in invalid state, unregistering", c)
			if err := c.vm.Unregister(op); err != nil {
				op.Errorf("Error while attempting to unregister container VM %s: %s", c, err)
				return err
			}
		case *types.ConcurrentAccess:
			// We are getting ConcurrentAccess errors from DeleteExceptDisks - even though we don't set ChangeVersion in that path
			// We are ignoring the error because in reality the operation finishes successfully.
			op.Warnf("DeleteExceptDisks failed with ConcurrentAccess error for %s. Ignoring it.", c)
			concurrent = true
		default:
			op.Debugf("Unhandled fault while attempting to destroy vm %s: %#v", c, f.Fault())

			c.updateState(op, existingState)
			return err
		}
	}

	if concurrent && c.vm.IsVC() {
		if err := c.vm.Unregister(op); err != nil {
			if !IsNotFoundError(err) {
				op.Errorf("Error while attempting to unregister container VM %s: %s", c, err)
				return err
			}
		}
	}

	// remove from datastore
	fm := ds.NewFileManager(sess.Datacenter, true)
	if err = fm.Delete(op, url.Path); err != nil {
		// at this phase error doesn't matter. Just log it.
		op.Debugf("Failed to delete %s, %s for %s", url, err, c)
	}

	//remove container from cache
	Containers.Remove(c.ExecConfig.ID)
	publishContainerEvent(op, c.ExecConfig.ID, time.Now(), events.ContainerRemoved)

	return nil
}

// eventedState will determine the target container
// state based on the current container state and the vsphere event
func eventedState(op trace.Operation, e events.Event, current State) State {
	switch e.String() {
	case events.ContainerPoweredOn:
		// are we in the process of starting
		if current != StateStarting {
			return StateRunning
		}
	case events.ContainerPoweredOff:
		// are we in the process of stopping or just created
		if current != StateStopping && current != StateCreated {
			return StateStopped
		}
	case events.ContainerSuspended:
		// are we in the process of suspending
		if current != StateSuspending {
			return StateSuspended
		}
	case events.ContainerRemoved:
		if current != StateRemoving {
			return StateRemoved
		}
	}

	return current
}

func (c *Container) OnEvent(e events.Event) {
	op := trace.NewOperation(context.Background(), "OnEvent")
	defer trace.End(trace.Begin(fmt.Sprintf("eventID(%s) received for event: %s", e.EventID(), e.String()), op))
	c.m.Lock()
	defer c.m.Unlock()

	if c.vm == nil {
		op.Warnf("Event(%s) received for %s but no VM found", e.EventID(), e.Reference())
		return
	}

	newState := eventedState(op, e, c.state)
	c.onEvent(op, newState, e)
}

// determine if the containerVM has started - this could pick up stale data in the started field for an out-of-band
// power change such as HA or user intervention where we have not had an opportunity to reset the entry.
func cleanStart(op trace.Operation, c *Container) bool {
	if len(c.ExecConfig.Sessions) == 0 {
		op.Warnf("Container %c has no sessions stored in in-memory config", c.ExecConfig.ID)
		// if no sessions, then nothing to wait for
		return true
	}

	for _, session := range c.ExecConfig.Sessions {
		if session.Started != "true" {
			return false
		}
	}
	return true
}

// onEvent determines what needs to be done when receiving a state update. It filters duplicate state transitions
// and publishes container events as needed in addition to performing necessary manipulations.
// newState - this is the new state determined by eventedState
// e - the source event used to derive the new State and reason for the transition
func (c *Container) onEvent(op trace.Operation, newState State, e events.Event) {
	// does local data report full start
	started := cleanStart(op, c)
	// do we need a refresh
	refresh := e.String() == events.ContainerRelocated
	// if it's a state event we've already done a refresh to end up here and dont need another
	_, stateEvent := e.(*stateevents.StateEvent)
	// the event we're going to publish - may be overridden/transformed by more context aware logic below
	// the incoming event is from the very coarse vSphere events
	publishEventType := e.String()

	if !stateEvent {
		if (newState == StateStarting && !started) || newState == StateStopping {
			// inherently transient state. Starting with started == true is just accounting that will
			// happen below and doesn't need a refresh.
			refresh = true
		}

		if newState == StateRunning && !started {
			// if we cannot confirm fully initialized
			refresh = true
		}
	}

	if refresh {
		op, cancel := trace.WithTimeout(&op, constants.PropertyCollectorTimeout, "vSphere event triggered refresh")
		defer cancel()

		if err := c.refresh(op); err != nil {
			op.Errorf("Container(%s) event driven update failed: %s", c, err)
		}
	}

	started = cleanStart(op, c)
	// it doesn't matter how the event was translated, if we're not fully started then we're starting
	// if we are then we're running. Only exception is that we don't transition from Running->Starting
	if newState == StateRunning && !started && c.state != StateRunning {
		newState = StateStarting
	}
	if newState == StateStarting && started {
		newState = StateRunning
	}

	if newState != c.state {
		switch newState {
		case StateRunning:
			// transform the PoweredOn event into Started
			publishEventType = events.ContainerStarted
			fallthrough

		case StateStarting,
			StateStopping,
			StateStopped,
			StateSuspended:

			c.updateState(op, newState)
			if newState == StateStopped {
				c.onStop()
			}
		case StateRemoved:
			if c.vm != nil && c.vm.IsFixing() {
				// is fixing vm, which will be registered back soon, so do not remove from containers cache
				op.Debugf("Container(%s) %s is being fixed - %s event ignored", c, newState)

				// Received remove event triggered by unregister VM operation - leave
				// fixing state now. In a loaded environment, the remove event may be
				// received after vm.fixVM() has returned, at which point the container
				// should still be in fixing state to avoid removing it from the cache below.
				c.vm.LeaveFixingState()
				// since we're leaving the container in cache, just return w/o allowing
				// a container event to be propogated to subscribers
				return
			}
			op.Debugf("Container(%s) %s via event activity", c, newState)
			// if we are here the containerVM has been removed from vSphere, so lets remove it
			// from the portLayer cache
			Containers.Remove(c.ExecConfig.ID)
			c.vm = nil
		default:
			return
		}

		op.Debugf("Container (%s) publishing event (state=%s, event=%s) from event %s", c, newState, publishEventType, e.String())
		// regardless of state update success or failure publish the container event
		publishContainerEvent(op, c.ExecConfig.ID, e.Created(), publishEventType)
		return
	}
}

// get the containerVMs from infrastructure for this resource pool
func infraContainers(ctx context.Context, sess *session.Session) ([]*Container, error) {
	defer trace.End(trace.Begin(""))
	var rp mo.ResourcePool

	// popluate the vm property of the vch resource pool
	if err := Config.ResourcePool.Properties(ctx, Config.ResourcePool.Reference(), []string{"vm"}, &rp); err != nil {
		name := Config.ResourcePool.Name()
		log.Errorf("List failed to get %s resource pool child vms: %s", name, err)
		return nil, err
	}
	vms, err := populateVMAttributes(ctx, sess, rp.Vm)
	if err != nil {
		return nil, err
	}

	return convertInfraContainers(ctx, sess, vms), nil
}

func instanceUUID(id string) (string, error) {
	// generate VM instance uuid, which will be used to query back VM
	u, err := sys.UUID()
	if err != nil {
		return "", err
	}
	namespace, err := uuid.Parse(u)
	if err != nil {
		return "", errors.Errorf("unable to parse VCH uuid: %s", err)
	}
	return uuid.NewSHA1(namespace, []byte(id)).String(), nil
}

// populate the vm attributes for the specified morefs
func populateVMAttributes(ctx context.Context, sess *session.Session, refs []types.ManagedObjectReference) ([]mo.VirtualMachine, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("populating %d refs", len(refs))))
	var vms []mo.VirtualMachine

	// current attributes we care about
	attrib := []string{"config", "runtime.powerState", "summary"}

	// populate the vm properties
	err := sess.Retrieve(ctx, refs, attrib, &vms)
	return vms, err
}

// convert the infra containers to a container object
func convertInfraContainers(ctx context.Context, sess *session.Session, vms []mo.VirtualMachine) []*Container {
	defer trace.End(trace.Begin(fmt.Sprintf("converting %d containers", len(vms))))
	var cons []*Container

	for _, v := range vms {
		vm := vm.NewVirtualMachine(ctx, sess, v.Reference())
		base := newBase(vm, v.Config, &v.Runtime)
		c := newContainer(base)

		id := uid.Parse(c.ExecConfig.ID)
		if id == uid.NilUID {
			log.Warnf("skipping converting container VM %s: could not parse id", v.Reference())
			continue
		}

		if v.Summary.Storage != nil {
			c.VMUnsharedDisk = v.Summary.Storage.Unshared
		}

		cons = append(cons, c)
	}

	return cons
}
