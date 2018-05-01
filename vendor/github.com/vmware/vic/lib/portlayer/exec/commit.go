// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/portlayer/event/events"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// Commit executes the requires steps on the handle
func Commit(op trace.Operation, sess *session.Session, h *Handle, waitTime *int32) error {
	defer trace.End(trace.Begin(h.ExecConfig.ID, op))

	c := Containers.Container(h.ExecConfig.ID)
	creation := h.vm == nil
	if creation {
		if h.Spec == nil {
			return fmt.Errorf("a spec must be provided for create operations")
		}

		if sess == nil {
			// session must not be nil
			return fmt.Errorf("no session provided for create operations")
		}

		// the only permissible operation is to create a VM
		if h.Spec == nil {
			return fmt.Errorf("only create operations can be committed without an existing VM")
		}

		if c != nil {
			return fmt.Errorf("a container already exists in the cache with this ID")
		}

		var res *types.TaskInfo
		var err error
		if sess.IsVC() && Config.VirtualApp.ResourcePool != nil {
			// Create the vm
			res, err = tasks.WaitForResult(op, func(op context.Context) (tasks.Task, error) {
				return Config.VirtualApp.CreateChildVM(op, *h.Spec.Spec(), nil)
			})
		} else {
			// Create the vm
			res, err = tasks.WaitForResult(op, func(op context.Context) (tasks.Task, error) {
				return sess.VMFolder.CreateVM(op, *h.Spec.Spec(), Config.ResourcePool, nil)
			})
		}

		if err != nil {
			op.Errorf("An error occurred while waiting for a creation operation to complete. Spec was %+v", *h.Spec.Spec())
			return err
		}

		h.vm = vm.NewVirtualMachine(op, sess, res.Result.(types.ManagedObjectReference))
		h.vm.DisableDestroy(op)
		c = newContainer(&h.containerBase)
		Containers.Put(c)
		// inform of creation irrespective of remaining operations
		publishContainerEvent(op, c.ExecConfig.ID, time.Now().UTC(), events.ContainerCreated)

		// clear the spec as we've acted on it - this prevents a reconfigure from occurring in follow-on
		// processing
		h.Spec = nil
	}

	// if we're stopping the VM, do so before the reconfigure to preserve the extraconfig
	if h.TargetState() == StateStopped {
		if h.Runtime == nil {
			op.Warnf("Commit called with incomplete runtime state for %s", h.ExecConfig.ID)
		}

		if h.Runtime != nil && h.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOff {
			op.Infof("Dropping duplicate power off operation for %s", h.ExecConfig.ID)
		} else {
			// stop the container
			if err := c.stop(op, waitTime); err != nil {
				return err
			}

			// we must refresh now to get the new ChangeVersion - this is used to gate on powerstate in the reconfigure
			// because we cannot set the ExtraConfig if the VM is powered on. There is still a race here unfortunately because
			// tasks don't appear to contain the new ChangeVersion
			h.refresh(op)

			// inform of state change irrespective of remaining operations - but allow remaining operations to complete first
			// to avoid data race on container config
			defer publishContainerEvent(op, h.ExecConfig.ID, time.Now().UTC(), events.ContainerStopped)
		}
	}

	// reconfigure operation
	if h.Spec != nil {
		if h.Runtime == nil {
			op.Errorf("Refusing to perform reconfigure operation with incomplete runtime state for %s", h.ExecConfig.ID)
		} else {
			// ensure that our logic based on Runtime state remains valid

			// NOTE: this inline refresh can be removed when switching away from guestinfo where we have non-persistence issues
			// when updating ExtraConfig via the API with a powered on VM - we therefore have to be absolutely certain about the
			// power state to decide if we can continue without nilifying extraconfig
			//
			// For the power off path this depends on handle.refresh() having been called to update the ChangeVersion
			s := h.Spec.Spec()

			op.Infof("Reconfigure: attempting update to %s with change version %q (%s)", h.ExecConfig.ID, s.ChangeVersion, h.Runtime.PowerState)

			// nilify ExtraConfig if container configuration is migrated
			// in this case, VCH and container are in different version. Migrated configuration cannot be written back to old container, to avoid data loss in old version's container
			if h.Migrated {
				op.Debugf("Reconfigure: dropping extraconfig as configuration of container %s is migrated", h.ExecConfig.ID)
				s.ExtraConfig = nil
			}

			// address the race between power operation and refresh of config (and therefore ChangeVersion) in StateStopped block above
			if s.ExtraConfig != nil && h.TargetState() == StateStopped && h.Runtime.PowerState != types.VirtualMachinePowerStatePoweredOff {
				detail := fmt.Sprintf("Reconfigure: collision of concurrent operations - expected power state poweredOff, found %s", h.Runtime.PowerState)
				op.Warnf(detail)

				// log out current vm power state and runtime power state got from refresh, to see if there is anything mismatch,
				// cause in issue #6127, we see the runtime power state is not updated even after 1 minute
				ps, _ := h.vm.PowerState(op)
				op.Debugf("Container %s power state: %s, runtime power state: %s", h.ExecConfig.ID, ps, h.Runtime.PowerState)
				// this should cause a second attempt at the power op. This could result repeated contention that fails to resolve, but the randomness in the backoff and the tight timing
				// to hit this scenario should mean it will resolve in a reasonable timeframe.
				return ConcurrentAccessError{errors.New(detail)}
			}

			_, err := h.vm.WaitForResult(op, func(op context.Context) (tasks.Task, error) {
				return h.vm.Reconfigure(op, *s)
			})
			if err != nil {
				op.Errorf("Reconfigure: failed update to %s with change version %s: %+v", h.ExecConfig.ID, s.ChangeVersion, err)

				// Check whether we get ConcurrentAccess and wrap it if needed
				if f, ok := err.(types.HasFault); ok {
					switch f.Fault().(type) {
					case *types.ConcurrentAccess:
						op.Errorf("Reconfigure: failed update to %s due to ConcurrentAccess, our change version %s", h.ExecConfig.ID, s.ChangeVersion)

						return ConcurrentAccessError{err}
					}
				}
				return err
			}

			op.Infof("Reconfigure: committed update to %s with change version: %s", h.ExecConfig.ID, s.ChangeVersion)

			// trigger a configuration reload in the container if needed
			err = reloadConfig(op, h, c)
			if err != nil {
				return err
			}
		}
	}

	// best effort update of container cache using committed state - this will not reflect the power on below, however
	// this is primarily for updating ExtraConfig state.
	if !creation {
		defer c.RefreshFromHandle(op, h)
	}

	if h.TargetState() == StateRunning {
		if h.Runtime != nil && h.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn {
			op.Infof("Dropping duplicate power on operation for %s", h.ExecConfig.ID)
			return nil
		}

		if h.Runtime == nil && !creation {
			op.Warnf("Commit called with incomplete runtime state for %s", h.ExecConfig.ID)
		}

		// start the container
		if err := c.start(op); err != nil {
			// We observed that PowerOn_Task could get stuck on VC time to time even though the VM was starting fine on the host ESXi.
			// Eventually the task was getting timed out (After 20 min.) and that was setting the container state back to Stopped.
			// During that time VC was not generating any other event so the persona listener was getting nothing.
			// This new event is for signaling the eventmonitor so that it can autoremove the container after this failure.
			publishContainerEvent(op, h.ExecConfig.ID, time.Now().UTC(), events.ContainerFailed)
			return err
		}

		// publish started event
		publishContainerEvent(op, h.ExecConfig.ID, time.Now().UTC(), events.ContainerStarted)
	}

	return nil
}

// HELPER FUNCTIONS BELOW

// reloadConfig is responsible for triggering a guest_reconfigure in order to perform an operation on a running cVM
// this function needs to be resilient to intermittent config errors and task errors, but will pass concurrent
// modification issues back immediately.
func reloadConfig(op trace.Operation, h *Handle, c *Container) error {

	op.Infof("Attempting to perform a guest reconfigure operation on (%s)", h.ExecConfig.ID)
	retryFunc := func() error {
		if h.reload && h.Runtime != nil && h.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn {
			err := c.ReloadConfig(op)

			if err != nil {
				op.Debugf("Error occurred during an attempt to reload the container config for an exec operation: (%s)", err)

				// we will request the powerstate directly(this could be very costly without the vmomi gateway)
				state, err := c.vm.PowerState(op)
				if err != nil && state == types.VirtualMachinePowerStatePoweredOff {
					// TODO: probably should make this error a specific type such as PowerOffDuringExecError( or a better name ofcourse)
					return fmt.Errorf("container(%s) was powered down during the requested operation.", h.ExecConfig.ID)
				}
				return err
			}
			return nil
		}

		// nothing to be done.
		return nil
	}

	err := retry.Do(retryFunc, isIntermittentFailure)
	if err != nil {
		op.Debugf("Failed an exec operation with err: %s", err)
		return err
	}
	return nil
}

// TODO: refactor later, I need to test this and we need to unify the Task package and the retry Package(make task use retry imo)
// right now this just looks silly...
func isIntermittentFailure(err error) bool {
	// in the future commit should be using the trace.operation for these calls and this function can act as a passthrough.
	op := trace.NewOperation(context.TODO(), "")
	return tasks.IsRetryError(op, err)
}
