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

// Package tasks wraps the operation of VC. It will invoke the operation and wait
// until it's finished, and then return the execution result or error message.
package tasks

import (
	"context"
	"math/rand"
	"time"

	"github.com/vmware/govmomi/task"
	"github.com/vmware/govmomi/vim25/progress"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/trace"
)

const (
	maxBackoffFactor = int64(16)
)

//FIXME: remove this type and refactor to use object.Task from govmomi
//       this will require a lot of code being touched in a lot of places.
type Task interface {
	Wait(ctx context.Context) error
	WaitForResult(ctx context.Context, s progress.Sinker) (*types.TaskInfo, error)
}

type temporary interface {
	Temporary() bool
}

// Wait wraps govmomi operations and wait the operation to complete
// Sample usage:
//    info, err := Wait(ctx, func(ctx), (*object.Reference, *TaskInfo, error) {
//       return vm, vm.Reconfigure(ctx, config)
//    })
func Wait(ctx context.Context, f func(context.Context) (Task, error)) error {
	_, err := WaitForResult(ctx, f)
	return err
}

// WaitForResult wraps govmomi operations and wait the operation to complete.
// Return the operation result
// Sample usage:
//    info, err := WaitForResult(ctx, func(ctx) (*TaskInfo, error) {
//       return vm, vm.Reconfigure(ctx, config)
//    })
func WaitForResult(ctx context.Context, f func(context.Context) (Task, error)) (*types.TaskInfo, error) {
	var err error
	var backoffFactor int64 = 1

	op := trace.FromContext(ctx, "WaitForResult")

	for {
		var t Task
		var info *types.TaskInfo

		if t, err = f(op); err == nil {
			if info, err = t.WaitForResult(op, nil); err == nil {
				return info, nil
			}
		}

		if !IsRetryError(op, err) {
			return info, err
		}

		sleepValue := time.Duration(backoffFactor * (rand.Int63n(100) + int64(50)))
		select {
		case <-time.After(sleepValue * time.Millisecond):
			backoffFactor *= 2
			if backoffFactor > maxBackoffFactor {
				backoffFactor = maxBackoffFactor
			}
		case <-op.Done():
			return info, op.Err()
		}

		op.Warnf("retrying task")
	}
}

const (
	vimFault  = "vim"
	soapFault = "soap"
	taskFault = "task"
)

// IsRetryErrors will return true for vSphere errors, which can be fixed by retry.
// Currently the error includes TaskInProgress, NetworkDisruptedAndConfigRolledBack and InvalidArgument
// Retry on NetworkDisruptedAndConfigRolledBack is to workaround vSphere issue
// Retry on InvalidArgument(invlid path) is to workaround vSAN bug: https://bugzilla.eng.vmware.com/show_bug.cgi?id=1770798. TODO: Should remove it after vSAN fixed the bug
func IsRetryError(op trace.Operation, err error) bool {
	if soap.IsSoapFault(err) {
		switch f := soap.ToSoapFault(err).VimFault().(type) {
		case types.TaskInProgress:
			return true
		case types.NetworkDisruptedAndConfigRolledBack:
			logExpectedFault(op, soapFault, f)
			return true
		case types.InvalidArgument:
			logExpectedFault(op, soapFault, f)
			return true
		case types.VAppTaskInProgress:
			logExpectedFault(op, soapFault, f)
			return true
		case types.FailToLockFaultToleranceVMs:
			logExpectedFault(op, soapFault, f)
			return true
		case types.HostCommunication:
			logExpectedFault(op, soapFault, f)
			return true
		default:
			logSoapFault(op, f)
			return false
		}
	}

	if soap.IsVimFault(err) {
		switch f := soap.ToVimFault(err).(type) {
		case *types.TaskInProgress:
			return true
		case *types.NetworkDisruptedAndConfigRolledBack:
			logExpectedFault(op, vimFault, f)
			return true
		case *types.InvalidArgument:
			logExpectedFault(op, vimFault, f)
			return true
		case *types.VAppTaskInProgress:
			logExpectedFault(op, soapFault, f)
			return true
		case *types.FailToLockFaultToleranceVMs:
			logExpectedFault(op, soapFault, f)
			return true
		case *types.HostCommunication:
			logExpectedFault(op, soapFault, f)
			return true
		default:
			logFault(op, f)
			return false
		}
	}

	switch err := err.(type) {
	case task.Error:
		switch f := err.Fault().(type) {
		case *types.TaskInProgress:
			return true
		case *types.NetworkDisruptedAndConfigRolledBack:
			logExpectedFault(op, taskFault, f)
			return true
		case *types.InvalidArgument:
			logExpectedFault(op, taskFault, f)
			return true
		case *types.HostCommunication:
			logExpectedFault(op, taskFault, f)
			return true
		default:
			logFault(op, err.Fault())
			return false
		}
	default:
		// retry the temporary errors
		t, ok := err.(temporary)
		if ok && t.Temporary() {
			logExpectedError(op, err)
			return true
		}
		logError(op, err)
		return false
	}
}

// Helper Functions
func logFault(op trace.Operation, fault types.BaseMethodFault) {
	op.Errorf("unexpected fault on task retry: %#v", fault)
}

func logSoapFault(op trace.Operation, fault types.AnyType) {
	op.Debugf("unexpected soap fault on task retry: %s", fault)
}

func logError(op trace.Operation, err error) {
	op.Debugf("unexpected error on task retry: %s", err)
}

func logExpectedFault(op trace.Operation, kind string, fault interface{}) {
	op.Debugf("task retry on expected %s fault: %#v", kind, fault)
}

func logExpectedError(op trace.Operation, err error) {
	op.Debugf("task retry on expected error %s", err)
}
