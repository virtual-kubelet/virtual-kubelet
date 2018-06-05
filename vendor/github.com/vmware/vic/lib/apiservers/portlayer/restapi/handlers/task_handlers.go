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

package handlers

import (
	"context"
	"strings"

	"github.com/go-openapi/runtime/middleware"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations/tasks"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/lib/portlayer/task"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/uid"
)

// TaskHandlersImpl is the receiver for all of the task handler methods
type TaskHandlersImpl struct {
}

func (handler *TaskHandlersImpl) Configure(api *operations.PortLayerAPI, _ *HandlerContext) {
	api.TasksJoinHandler = tasks.JoinHandlerFunc(handler.JoinHandler)
	api.TasksBindHandler = tasks.BindHandlerFunc(handler.BindHandler)
	api.TasksUnbindHandler = tasks.UnbindHandlerFunc(handler.UnbindHandler)
	api.TasksRemoveHandler = tasks.RemoveHandlerFunc(handler.RemoveHandler)
	api.TasksInspectHandler = tasks.InspectHandlerFunc(handler.InspectHandler)
	api.TasksWaitHandler = tasks.WaitHandlerFunc(handler.WaitHandler)
}

// JoinHandler calls the Join
func (handler *TaskHandlersImpl) JoinHandler(params tasks.JoinParams) middleware.Responder {
	defer trace.End(trace.Begin(""))
	op := trace.NewOperation(context.Background(), "task.Join(%s, %s)", params.Config.Handle, params.Config.ID)

	handle := exec.HandleFromInterface(params.Config.Handle)
	if handle == nil {
		err := &models.Error{Message: "Failed to get the Handle"}
		return tasks.NewJoinInternalServerError().WithPayload(err)
	}

	// TODO: ensure uniqueness of ID - this is already an issue with containercreate now we're not using it as
	// the VM name and cannot rely on vSphere for uniqueness guarantee
	id := params.Config.ID
	if id == "" {
		id = uid.New().String()
	}

	op.Debugf("ID: %#v", id)
	op.Debugf("Path: %#v", params.Config.Path)
	op.Debugf("WorkingDir: %#v", params.Config.WorkingDir)
	op.Debugf("OpenStdin: %#v", params.Config.OpenStdin)
	op.Debugf("Attach: %#v", params.Config.Attach)

	op.Debugf("User: %s", params.Config.User)

	sessionConfig := &executor.SessionConfig{
		Common: executor.Common{
			ExecutionEnvironment: params.Config.Namespace,
			ID:                   id,
		},
		Tty:       params.Config.Tty,
		Attach:    params.Config.Attach,
		OpenStdin: params.Config.OpenStdin,
		Cmd: executor.Cmd{
			Env:  params.Config.Env,
			Dir:  params.Config.WorkingDir,
			Path: params.Config.Path,
			Args: append([]string{params.Config.Path}, params.Config.Args...),
		},
		StopSignal: params.Config.StopSignal,
	}

	// parsing user
	if params.Config.User != "" {
		parts := strings.Split(params.Config.User, ":")
		if len(parts) > 0 {
			sessionConfig.User = parts[0]
		}
		if len(parts) > 1 {
			sessionConfig.Group = parts[1]
		}
	}

	handleprime, err := task.Join(&op, handle, sessionConfig)
	if err != nil {
		op.Errorf("%s", err.Error())

		return tasks.NewJoinInternalServerError().WithPayload(
			&models.Error{Message: err.Error()},
		)
	}
	res := &models.TaskJoinResponse{
		ID:     id,
		Handle: exec.ReferenceFromHandle(handleprime),
	}
	return tasks.NewJoinOK().WithPayload(res)
}

// BindHandler calls the Bind
func (handler *TaskHandlersImpl) BindHandler(params tasks.BindParams) middleware.Responder {
	defer trace.End(trace.Begin(""))
	op := trace.NewOperation(context.Background(), "task.Bind(%s, %s)", params.Config.Handle, params.Config.ID)

	handle := exec.HandleFromInterface(params.Config.Handle)
	if handle == nil {
		err := &models.Error{Message: "Failed to get the Handle"}
		return tasks.NewBindInternalServerError().WithPayload(err)
	}

	handleprime, err := task.Bind(&op, handle, params.Config.ID)
	if err != nil {
		op.Errorf("%s", err.Error())

		switch err.(type) {
		case task.TaskNotFoundError:
			return tasks.NewBindNotFound().WithPayload(
				&models.Error{Message: err.Error()},
			)
		default:
			return tasks.NewBindInternalServerError().WithPayload(
				&models.Error{Message: err.Error()},
			)
		}

	}

	res := &models.TaskBindResponse{
		Handle: exec.ReferenceFromHandle(handleprime),
	}

	return tasks.NewBindOK().WithPayload(res)
}

// UnbindHandler calls the Unbind
func (handler *TaskHandlersImpl) UnbindHandler(params tasks.UnbindParams) middleware.Responder {
	defer trace.End(trace.Begin(""))
	op := trace.NewOperation(context.Background(), "task.Unbind(%s, %s)", params.Config.Handle, params.Config.ID)

	handle := exec.HandleFromInterface(params.Config.Handle)
	if handle == nil {
		err := &models.Error{Message: "Failed to get the Handle"}
		return tasks.NewUnbindInternalServerError().WithPayload(err)
	}

	handleprime, err := task.Unbind(&op, handle, params.Config.ID)
	if err != nil {
		op.Errorf("%s", err.Error())

		switch err.(type) {
		case task.TaskNotFoundError:
			return tasks.NewUnbindNotFound().WithPayload(
				&models.Error{Message: err.Error()},
			)
		default:
			return tasks.NewUnbindInternalServerError().WithPayload(
				&models.Error{Message: err.Error()},
			)
		}

	}

	res := &models.TaskUnbindResponse{
		Handle: exec.ReferenceFromHandle(handleprime),
	}
	return tasks.NewUnbindOK().WithPayload(res)
}

// RemoveHandler calls remove
func (handler *TaskHandlersImpl) RemoveHandler(params tasks.RemoveParams) middleware.Responder {
	defer trace.End(trace.Begin(""))
	op := trace.NewOperation(context.Background(), "task.Remove(%s, %s)", params.Config.Handle, params.Config.ID)

	handle := exec.HandleFromInterface(params.Config.Handle)
	if handle == nil {
		err := &models.Error{Message: "Failed to get the Handle"}
		return tasks.NewRemoveInternalServerError().WithPayload(err)
	}

	handleprime, err := task.Remove(&op, handle, params.Config.ID)
	if err != nil {
		op.Errorf("%s", err.Error())

		return tasks.NewRemoveInternalServerError().WithPayload(
			&models.Error{Message: err.Error()},
		)
	}

	res := &models.TaskRemoveResponse{
		Handle: exec.ReferenceFromHandle(handleprime),
	}
	return tasks.NewRemoveOK().WithPayload(res)
}

// InspectHandler calls inspect
func (handler *TaskHandlersImpl) InspectHandler(params tasks.InspectParams) middleware.Responder {
	defer trace.End(trace.Begin(""))
	op := trace.NewOperation(context.Background(), "task.Inspect(%s, %s)", params.Config.Handle, params.Config.ID)

	handle := exec.HandleFromInterface(params.Config.Handle)
	if handle == nil {
		err := &models.Error{Message: "Failed to get the Handle"}
		return tasks.NewInspectInternalServerError().WithPayload(err)
	}

	t, err := task.Inspect(&op, handle, params.Config.ID)
	if err != nil {
		op.Errorf("%s", err.Error())

		switch err.(type) {
		case task.TaskNotFoundError:
			return tasks.NewInspectNotFound().WithPayload(
				&models.Error{Message: err.Error()},
			)
		default:
			return tasks.NewInspectInternalServerError().WithPayload(
				&models.Error{Message: err.Error()},
			)
		}
	}

	op.Debugf("ID: %#v", t.ID)
	op.Debugf("Path: %#v", t.Cmd.Path)
	op.Debugf("Args: %#v", t.Cmd.Args)
	op.Debugf("Running: %#v", t.StartTime)
	op.Debugf("ExitCode: %#v", t.ExitStatus)

	res := &models.TaskInspectResponse{
		ID:       t.ID,
		Running:  t.Started == "true",
		ExitCode: int64(t.ExitStatus),
		ProcessConfig: &models.ProcessConfig{
			ExecPath: t.Cmd.Path,
			ExecArgs: t.Cmd.Args,
		},
		Tty:        t.Tty,
		User:       t.User,
		OpenStdin:  t.OpenStdin,
		OpenStdout: t.Attach,
		OpenStderr: t.Attach,
		Pid:        0,
	}

	// report launch error if we failed
	if t.Started != "" && t.Started != "true" {
		res.ProcessConfig.ErrorMsg = t.Started
	}

	return tasks.NewInspectOK().WithPayload(res)
}

// WaitHandler calls wait
func (handler *TaskHandlersImpl) WaitHandler(params tasks.WaitParams) middleware.Responder {
	defer trace.End(trace.Begin(""))
	op := trace.NewOperation(context.Background(), "task.Wait(%s, %s)", params.Config.Handle, params.Config.ID)

	handle := exec.HandleFromInterface(params.Config.Handle)
	if handle == nil {
		err := &models.Error{Message: "Failed to get the Handle"}
		return tasks.NewInspectInternalServerError().WithPayload(err)
	}

	// wait task to set started field to something
	err := task.Wait(&op, handle, params.Config.ID)
	if err != nil {
		switch err := err.(type) {
		case *task.TaskPowerStateError:
			op.Errorf("The container was in an invalid power state for the wait operation: %s", err.Error())
			return tasks.NewWaitPreconditionRequired().WithPayload(
				&models.Error{Message: err.Error()})
		case *task.TaskNotFoundError:
			op.Errorf("The task was unable to be found: %s", err.Error())
			return tasks.NewWaitNotFound().WithPayload(
				&models.Error{Message: err.Error()})
		default:
			op.Errorf("%s", err.Error())
			return tasks.NewWaitInternalServerError().WithPayload(
				&models.Error{Message: err.Error()})
		}
	}

	return tasks.NewWaitOK()
}
