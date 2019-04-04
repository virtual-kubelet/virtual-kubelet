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

package proxy

//****
// container_proxy.go
//
// Contains all code that touches the portlayer for container operations and all
// code that converts swagger based returns to docker personality backend structs.
// The goal is to make the backend code that implements the docker engine-api
// interfaces be as simple as possible and contain no swagger or portlayer code.
//
// Rule for code to be in here:
// 1. touches VIC portlayer
// 2. converts swagger to docker engine-api structs
// 3. errors MUST be docker engine-api compatible errors.  DO NOT return arbitrary errors!
//		- Do NOT return portlayer errors
//		- Do NOT return fmt.Errorf()
//		- Do NOT return errors.New()
//		- Please USE the aliased docker error package 'derr'

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"

	derr "github.com/docker/docker/api/errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	dnetwork "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/go-connections/nat"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	viccontainer "github.com/vmware/vic/lib/apiservers/engine/backends/container"
	"github.com/vmware/vic/lib/apiservers/engine/backends/convert"
	epoint "github.com/vmware/vic/lib/apiservers/engine/backends/endpoint"
	"github.com/vmware/vic/lib/apiservers/engine/backends/filter"
	engconstants "github.com/vmware/vic/lib/apiservers/engine/constants"
	"github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/engine/network"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/containers"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/interaction"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/logging"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/scopes"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/storage"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/tasks"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/metadata"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/sys"
)

// VicContainerProxy interface
type ContainerProxy interface {
	CreateContainerHandle(ctx context.Context, vc *viccontainer.VicContainer, config types.ContainerCreateConfig) (string, string, error)
	AddImageToContainer(ctx context.Context, handle string, deltaID string, layerID string, imageID string, config types.ContainerCreateConfig) (string, error)
	CreateContainerTask(ctx context.Context, handle string, id string, layerID string, config types.ContainerCreateConfig) (string, error)
	CreateExecTask(ctx context.Context, handle string, config *types.ExecConfig) (string, string, error)
	AddContainerToScope(ctx context.Context, handle string, config types.ContainerCreateConfig) (string, error)
	AddLoggingToContainer(ctx context.Context, handle string, config types.ContainerCreateConfig) (string, error)
	AddInteractionToContainer(ctx context.Context, handle string, config types.ContainerCreateConfig) (string, error)

	BindInteraction(ctx context.Context, handle string, name string, id string) (string, error)
	UnbindInteraction(ctx context.Context, handle string, name string, id string) (string, error)
	UnbindContainerFromNetwork(ctx context.Context, vc *viccontainer.VicContainer, handle string) (string, error)
	CommitContainerHandle(ctx context.Context, handle, containerID string, waitTime int32) error

	// TODO: we should not be returning a swagger model here, however we do not have a solid architected return for this yet.
	InspectTask(op trace.Operation, handle string, eid string, cid string) (*models.TaskInspectResponse, error)
	BindTask(op trace.Operation, handle string, eid string) (string, error)
	WaitTask(op trace.Operation, handle string, cid string, eid string) error

	Handle(ctx context.Context, id, name string) (string, error)

	Stop(ctx context.Context, vc *viccontainer.VicContainer, name string, seconds *int, unbound bool) error
	State(ctx context.Context, vc *viccontainer.VicContainer) (*types.ContainerState, error)
	GetStateFromHandle(op trace.Operation, handle string) (string, string, error)
	Wait(ctx context.Context, vc *viccontainer.VicContainer, timeout time.Duration) (*types.ContainerState, error)
	Signal(ctx context.Context, vc *viccontainer.VicContainer, sig uint64) error
	Resize(ctx context.Context, id string, height, width int32) error
	Rename(ctx context.Context, vc *viccontainer.VicContainer, newName string) error
	Remove(ctx context.Context, vc *viccontainer.VicContainer, config *types.ContainerRmConfig) error

	ExitCode(ctx context.Context, vc *viccontainer.VicContainer) (string, error)
}

// ContainerProxy struct
type VicContainerProxy struct {
	client        *client.PortLayer
	portlayerAddr string
	portlayerName string
}

const (
	forceLogType = "json-file" //Use in inspect to allow docker logs to work
	ShortIDLen   = 12

	ContainerRunning = "running"
	ContainerError   = "error"
	ContainerStopped = "stopped"
	ContainerExited  = "exited"
	ContainerCreated = "created"
)

// NewContainerProxy will create a new proxy
func NewContainerProxy(plClient *client.PortLayer, portlayerAddr string, portlayerName string) *VicContainerProxy {
	return &VicContainerProxy{client: plClient, portlayerAddr: portlayerAddr, portlayerName: portlayerName}
}

// Handle retrieves a handle to a VIC container.  Handles should be treated as opaque strings.
//
// returns:
//	(handle string, error)
func (c *VicContainerProxy) Handle(ctx context.Context, id, name string) (string, error) {
	op := trace.FromContext(ctx, "Handle: %s", id)
	defer trace.End(trace.Begin(name, op))
	opID := op.ID()

	if c.client == nil {
		return "", errors.NillPortlayerClientError("ContainerProxy")
	}

	resp, err := c.client.Containers.Get(containers.NewGetParamsWithContext(ctx).WithOpID(&opID).WithID(id))
	if err != nil {
		switch err := err.(type) {
		case *containers.GetNotFound:
			cache.ContainerCache().DeleteContainer(id)
			return "", errors.NotFoundError(name)
		case *containers.GetDefault:
			return "", errors.InternalServerError(err.Payload.Message)
		default:
			return "", errors.InternalServerError(err.Error())
		}
	}
	return resp.Payload, nil
}

// CreateContainerHandle creates a new VIC container by calling the portlayer
//
// returns:
//	(containerID, containerHandle, error)
func (c *VicContainerProxy) CreateContainerHandle(ctx context.Context, vc *viccontainer.VicContainer, config types.ContainerCreateConfig) (string, string, error) {
	op := trace.FromContext(ctx, "CreateContainerHandle: %s", vc.Name)
	defer trace.End(trace.Begin(vc.Name, op))
	opID := op.ID()

	if c.client == nil {
		return "", "", errors.NillPortlayerClientError("ContainerProxy")
	}

	if vc.ImageID == "" {
		return "", "", errors.NotFoundError("No image specified")
	}

	if vc.LayerID == "" {
		return "", "", errors.NotFoundError("No layer specified")
	}

	// Call the Exec port layer to create the container
	host, err := sys.UUID()
	if err != nil {
		return "", "", errors.InternalServerError("ContainerProxy.CreateContainerHandle got unexpected error getting VCH UUID")
	}

	plCreateParams := dockerContainerCreateParamsToPortlayer(ctx, config, vc, host).WithOpID(&opID)
	createResults, err := c.client.Containers.Create(plCreateParams)
	if err != nil {
		if _, ok := err.(*containers.CreateNotFound); ok {
			cerr := fmt.Errorf("No such image: %s", vc.ImageID)
			op.Errorf("%s (%s)", cerr, err)
			return "", "", errors.NotFoundError(cerr.Error())
		}

		// If we get here, most likely something went wrong with the port layer API server
		return "", "", errors.InternalServerError(err.Error())
	}

	id := createResults.Payload.ID
	h := createResults.Payload.Handle

	return id, h, nil
}

// AddImageToContainer adds the specified image to a container, referenced by handle.
// If an error is return, the returned handle should not be used.
//
// returns:
//	modified handle
func (c *VicContainerProxy) AddImageToContainer(ctx context.Context, handle, deltaID, layerID, imageID string, config types.ContainerCreateConfig) (string, error) {
	defer trace.End(trace.Begin(handle))

	if c.client == nil {
		return "", errors.NillPortlayerClientError("ContainerProxy")
	}

	host, err := sys.UUID()
	if err != nil {
		return "", errors.InternalServerError("ContainerProxy.AddImageToContainer got unexpected error getting VCH UUID")
	}

	response, err := c.client.Storage.ImageJoin(storage.NewImageJoinParamsWithContext(ctx).WithStoreName(host).WithID(layerID).
		WithConfig(&models.ImageJoinConfig{
			Handle:   handle,
			DeltaID:  deltaID,
			ImageID:  imageID,
			RepoName: config.Config.Image,
		}))
	if err != nil {
		return "", errors.InternalServerError(err.Error())
	}
	handle, ok := response.Payload.Handle.(string)
	if !ok {
		return "", errors.InternalServerError(fmt.Sprintf("Type assertion failed for %#+v", handle))
	}

	return handle, nil
}

// CreateContainerTask sets the primary command to run in the container
//
// returns:
//	(containerHandle, error)
func (c *VicContainerProxy) CreateContainerTask(ctx context.Context, handle, id, layerID string, config types.ContainerCreateConfig) (string, error) {
	op := trace.FromContext(ctx, "CreateContainerTask: %s", id)
	defer trace.End(trace.Begin(id, op))
	opID := op.ID()

	if c.client == nil {
		return "", errors.NillPortlayerClientError("ContainerProxy")
	}

	plTaskParams := dockerContainerCreateParamsToTask(op, id, layerID, config)
	plTaskParams.Config.Handle = handle
	plTaskParams.WithOpID(&opID)

	op.Infof("*** CreateContainerTask - params = %#v", *plTaskParams.Config)
	responseJoin, err := c.client.Tasks.Join(plTaskParams)
	if err != nil {
		op.Errorf("Unable to join primary task to container: %+v", err)
		return "", errors.InternalServerError(err.Error())
	}

	handle, ok := responseJoin.Payload.Handle.(string)
	if !ok {
		return "", errors.InternalServerError(fmt.Sprintf("Type assertion failed on handle from task join: %#+v", handle))
	}

	plBindParams := tasks.NewBindParamsWithContext(ctx).
		WithOpID(&opID).
		WithConfig(&models.TaskBindConfig{Handle: handle, ID: id})

	responseBind, err := c.client.Tasks.Bind(plBindParams)
	if err != nil {
		op.Errorf("Unable to bind primary task to container: %+v", err)
		return "", errors.InternalServerError(err.Error())
	}

	handle, ok = responseBind.Payload.Handle.(string)
	if !ok {
		return "", errors.InternalServerError(fmt.Sprintf("Type assertion failed on handle from task bind %#+v", handle))
	}

	return handle, nil
}

func (c *VicContainerProxy) CreateExecTask(ctx context.Context, handle string, config *types.ExecConfig) (string, string, error) {
	op := trace.FromContext(ctx, "CreateExecTask: %s", handle)
	defer trace.End(trace.Begin(handle, op))
	opID := op.ID()

	if c.client == nil {
		return "", "", errors.NillPortlayerClientError("ContainerProxy")
	}

	joinconfig := &models.TaskJoinConfig{
		Handle:    handle,
		Path:      config.Cmd[0],
		Args:      config.Cmd[1:],
		Env:       config.Env,
		User:      config.User,
		Attach:    config.AttachStdin || config.AttachStdout || config.AttachStderr,
		OpenStdin: config.AttachStdin,
		Tty:       config.Tty,
	}

	// call Join with JoinParams
	joinparams := tasks.NewJoinParamsWithContext(ctx).WithOpID(&opID).WithConfig(joinconfig)
	resp, err := c.client.Tasks.Join(joinparams)
	if err != nil {
		return "", "", errors.InternalServerError(err.Error())
	}
	eid := resp.Payload.ID

	handleprime, ok := resp.Payload.Handle.(string)
	if !ok {
		return "", "", errors.InternalServerError(fmt.Sprintf("Type assertion failed on handle from task bind %#+v", handleprime))
	}

	return handleprime, eid, nil
}

// AddContainerToScope adds a container, referenced by handle, to a scope.
// If an error is return, the returned handle should not be used.
//
// returns:
//	modified handle
func (c *VicContainerProxy) AddContainerToScope(ctx context.Context, handle string, config types.ContainerCreateConfig) (string, error) {
	op := trace.FromContext(ctx, "AddContainerToScope: %s", handle)
	defer trace.End(trace.Begin(handle, op))
	opID := op.ID()

	if c.client == nil {
		return "", errors.NillPortlayerClientError("ContainerProxy")
	}

	op.Debugf("Network Configuration Section - Container Create")
	// configure network
	netConf := toModelsNetworkConfig(config)
	if netConf != nil {
		addContRes, err := c.client.Scopes.AddContainer(scopes.NewAddContainerParamsWithContext(ctx).
			WithOpID(&opID).
			WithScope(netConf.NetworkName).
			WithConfig(&models.ScopesAddContainerConfig{
				Handle:        handle,
				NetworkConfig: netConf,
			}))

		if err != nil {
			op.Errorf("ContainerProxy.AddContainerToScope: Scopes error: %s", err.Error())
			return handle, errors.InternalServerError(err.Error())
		}

		defer func() {
			if err == nil {
				return
			}
			// roll back the AddContainer call
			if _, err2 := c.client.Scopes.RemoveContainer(scopes.NewRemoveContainerParamsWithContext(ctx).
				WithHandle(handle).
				WithScope(netConf.NetworkName).
				WithOpID(&opID)); err2 != nil {
				op.Warnf("could not roll back container add: %s", err2)
			}
		}()

		handle = addContRes.Payload
	}

	return handle, nil
}

// AddLoggingToContainer adds logging capability to a container, referenced by handle.
// If an error is return, the returned handle should not be used.
//
// returns:
//	modified handle
func (c *VicContainerProxy) AddLoggingToContainer(ctx context.Context, handle string, config types.ContainerCreateConfig) (string, error) {
	op := trace.FromContext(ctx, "AddLoggingToContainer: %s", handle)
	defer trace.End(trace.Begin(handle, op))
	opID := op.ID()

	if c.client == nil {
		return "", errors.NillPortlayerClientError("ContainerProxy")
	}

	response, err := c.client.Logging.LoggingJoin(logging.NewLoggingJoinParamsWithContext(ctx).
		WithOpID(&opID).
		WithConfig(&models.LoggingJoinConfig{
			Handle: handle,
		}))
	if err != nil {
		return "", errors.InternalServerError(err.Error())
	}
	handle, ok := response.Payload.Handle.(string)
	if !ok {
		return "", errors.InternalServerError(fmt.Sprintf("Type assertion failed for %#+v", handle))
	}

	return handle, nil
}

// AddInteractionToContainer adds interaction capabilities to a container, referenced by handle.
// If an error is return, the returned handle should not be used.
//
// returns:
//	modified handle
func (c *VicContainerProxy) AddInteractionToContainer(ctx context.Context, handle string, config types.ContainerCreateConfig) (string, error) {
	op := trace.FromContext(ctx, "AddLoggingToContainer: %s", handle)
	defer trace.End(trace.Begin(handle, op))
	opID := op.ID()

	if c.client == nil {
		return "", errors.NillPortlayerClientError("ContainerProxy")
	}

	response, err := c.client.Interaction.InteractionJoin(interaction.NewInteractionJoinParamsWithContext(ctx).
		WithOpID(&opID).
		WithConfig(&models.InteractionJoinConfig{
			Handle: handle,
		}))
	if err != nil {
		return "", errors.InternalServerError(err.Error())
	}
	handle, ok := response.Payload.Handle.(string)
	if !ok {
		return "", errors.InternalServerError(fmt.Sprintf("Type assertion failed for %#+v", handle))
	}

	return handle, nil
}

// BindInteraction enables interaction capabilities
func (c *VicContainerProxy) BindInteraction(ctx context.Context, handle string, name string, id string) (string, error) {
	op := trace.FromContext(ctx, "BindInteraction: %s", handle)
	defer trace.End(trace.Begin(handle, op))
	opID := op.ID()

	if c.client == nil {
		return "", errors.NillPortlayerClientError("ContainerProxy")
	}

	bind, err := c.client.Interaction.InteractionBind(
		interaction.NewInteractionBindParamsWithContext(ctx).
			WithOpID(&opID).
			WithConfig(&models.InteractionBindConfig{
				Handle: handle,
				ID:     id,
			}))
	if err != nil {
		switch err := err.(type) {
		case *interaction.InteractionBindInternalServerError:
			return "", errors.InternalServerError(err.Payload.Message)
		default:
			return "", errors.InternalServerError(err.Error())
		}
	}
	handle, ok := bind.Payload.Handle.(string)
	if !ok {
		return "", errors.InternalServerError(fmt.Sprintf("Type assertion failed for %#+v", handle))
	}
	return handle, nil
}

// UnbindInteraction disables interaction capabilities
func (c *VicContainerProxy) UnbindInteraction(ctx context.Context, handle string, name string, id string) (string, error) {
	op := trace.FromContext(ctx, "UnbindInteraction: %s", handle)
	defer trace.End(trace.Begin(handle, op))
	opID := op.ID()

	if c.client == nil {
		return "", errors.NillPortlayerClientError("ContainerProxy")
	}

	unbind, err := c.client.Interaction.InteractionUnbind(
		interaction.NewInteractionUnbindParamsWithContext(ctx).
			WithOpID(&opID).
			WithConfig(&models.InteractionUnbindConfig{
				Handle: handle,
				ID:     id,
			}))
	if err != nil {
		return "", errors.InternalServerError(err.Error())
	}
	handle, ok := unbind.Payload.Handle.(string)
	if !ok {
		return "", errors.InternalServerError("type assertion failed")
	}

	return handle, nil
}

// CommitContainerHandle commits any changes to container handle.
//
// Args:
//	waitTime <= 0 means no wait time
func (c *VicContainerProxy) CommitContainerHandle(ctx context.Context, handle, containerID string, waitTime int32) error {
	op := trace.FromContext(ctx, "CommitContainerHandle: %s", handle)
	defer trace.End(trace.Begin(handle, op))
	opID := op.ID()

	if c.client == nil {
		return errors.NillPortlayerClientError("ContainerProxy")
	}

	commitParams := containers.NewCommitParamsWithContext(ctx).
		WithOpID(&opID).
		WithHandle(handle)

	if waitTime > 0 {
		commitParams.WithWaitTime(&waitTime)
	}

	_, err := c.client.Containers.Commit(commitParams)
	if err != nil {
		switch err := err.(type) {
		case *containers.CommitNotFound:
			return errors.NotFoundError(containerID)
		case *containers.CommitConflict:
			return errors.ConflictError(err.Error())
		case *containers.CommitDefault:
			return errors.InternalServerError(err.Payload.Message)
		default:
			return errors.InternalServerError(err.Error())
		}
	}

	return nil
}

func (c *VicContainerProxy) InspectTask(op trace.Operation, handle string, eid string, cid string) (*models.TaskInspectResponse, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("handle(%s), eid(%s), cid(%s)", handle, eid, cid), op))
	opID := op.ID()

	if c.client == nil {
		return nil, errors.NillPortlayerClientError("ContainerProxy")
	}

	// inspect the Task to obtain ProcessConfig
	config := &models.TaskInspectConfig{
		Handle: handle,
		ID:     eid,
	}

	// FIXME: right now we are only using this path for exec targets. But later the error messages may need to be changed
	// to be more accurate.
	params := tasks.NewInspectParamsWithContext(op).WithOpID(&opID).WithConfig(config)
	resp, err := c.client.Tasks.Inspect(params)
	if err != nil {
		switch err := err.(type) {
		case *tasks.InspectNotFound:
			// These error types may need to be expanded. NotFoundError does not fit here.
			op.Errorf("received a TaskNotFound error during task inspect: %s", err.Payload.Message)
			return nil, errors.TaskPoweredOffError(cid)
		case *tasks.InspectInternalServerError:
			op.Errorf("received an internal server error during task inspect: %s", err.Payload.Message)
			return nil, errors.InternalServerError(err.Payload.Message)
		default:
			// right now Task inspection in the portlayer does not return a conflict error
			return nil, errors.InternalServerError(err.Error())
		}
	}
	return resp.Payload, nil
}

func (c *VicContainerProxy) BindTask(op trace.Operation, handle string, eid string) (string, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("handle(%s), eid(%s)", handle, eid), op))
	opID := op.ID()

	if c.client == nil {
		return "", errors.NillPortlayerClientError("ContainerProxy")
	}

	bindconfig := &models.TaskBindConfig{
		Handle: handle,
		ID:     eid,
	}
	bindparams := tasks.NewBindParamsWithContext(op).WithOpID(&opID).WithConfig(bindconfig)

	// call Bind with bindparams
	resp, err := c.client.Tasks.Bind(bindparams)
	if err != nil {
		switch err := err.(type) {
		case *tasks.BindNotFound:
			op.Errorf("received TaskNotFound error during task bind: %s", err.Payload.Message)
			return "", errors.TaskBindPowerError()
		case *tasks.BindInternalServerError:
			op.Errorf("received unexpected error attempting to bind task(%s) for handle(%s): %s", eid, handle, err.Payload.Message)
			return "", errors.InternalServerError(err.Payload.Message)
		default:
			op.Errorf("received unexpected error attempting to bind task(%s) for handle(%s): %s", eid, handle, err.Error())
			return "", errors.InternalServerError(err.Error())
		}

	}

	respHandle, ok := resp.Payload.Handle.(string)
	if !ok {
		op.Errorf("Unable to marshal string object from BindTask response for handle(%s) on eid(%s)", handle, eid)
		// TODO: perhaps a better error message here?
		return "", errors.InternalServerError("An unknown error occurred during the handling of this request")
	}

	return respHandle, nil
}

func (c *VicContainerProxy) WaitTask(op trace.Operation, handle string, cid string, eid string) error {
	defer trace.End(trace.Begin(fmt.Sprintf("handle(%s), cid(%s)", handle, cid), op))
	opID := op.ID()

	if c.client == nil {
		return errors.NillPortlayerClientError("ContainerProxy")
	}

	// wait for the Task to change in state
	config := &models.TaskWaitConfig{
		Handle: handle,
		ID:     eid,
	}

	params := tasks.NewWaitParamsWithContext(op).WithOpID(&opID).WithConfig(config)
	_, err := c.client.Tasks.Wait(params)
	if err != nil {
		switch err := err.(type) {
		case *tasks.WaitNotFound:
			return errors.InternalServerError(fmt.Sprintf("the container(%s) has been shutdown during execution of the exec operation", cid))
		case *tasks.WaitPreconditionRequired:
			return errors.InternalServerError(fmt.Sprintf("container(%s) must be powered on in order to perform the desired exec operation", cid))
		case *tasks.WaitInternalServerError:
			return errors.InternalServerError(err.Payload.Message)
		default:
			return errors.InternalServerError(err.Error())
		}
	}

	return nil

}

// Stop will stop (shutdown) a VIC container.
//
// returns
//	error
func (c *VicContainerProxy) Stop(ctx context.Context, vc *viccontainer.VicContainer, name string, seconds *int, unbound bool) error {
	op := trace.FromContext(ctx, "Stop: %s", name)
	defer trace.End(trace.Begin(fmt.Sprintf("Name: %s, container id: %s", name, vc.ContainerID), op))
	opID := op.ID()

	if c.client == nil {
		return errors.NillPortlayerClientError("ContainerProxy")
	}

	//retrieve client to portlayer
	handle, err := c.Handle(context.TODO(), vc.ContainerID, name)
	if err != nil {
		return err
	}

	// we have a container on the PL side lets check the state before proceeding
	// ignore the error  since others will be checking below..this is an attempt to short circuit the op
	// TODO: can be replaced with simple cache check once power events are propagated to persona
	state, err := c.State(ctx, vc)
	if err != nil && errors.IsNotFoundError(err) {
		cache.ContainerCache().DeleteContainer(vc.ContainerID)
		return err
	}
	// attempt to stop container only if container state is not stopped, exited or created.
	// we should allow user to stop and remove the container that is in unexpected status, e.g. starting, because of serial port connection issue
	if state.Status == ContainerStopped || state.Status == ContainerExited || state.Status == ContainerCreated {
		return nil
	}

	if unbound {
		handle, err = c.UnbindContainerFromNetwork(ctx, vc, handle)
		if err != nil {
			return err
		}

		// unmap ports
		if err = network.UnmapPorts(vc.ContainerID, vc); err != nil {
			return err
		}
	}

	// change the state of the container
	changeParams := containers.NewStateChangeParamsWithContext(ctx).
		WithOpID(&opID).
		WithHandle(handle).
		WithState("STOPPED")

	stateChangeResponse, err := c.client.Containers.StateChange(changeParams)
	if err != nil {
		switch err := err.(type) {
		case *containers.StateChangeNotFound:
			cache.ContainerCache().DeleteContainer(vc.ContainerID)
			return errors.NotFoundError(name)
		case *containers.StateChangeDefault:
			return errors.InternalServerError(err.Payload.Message)
		default:
			return errors.InternalServerError(err.Error())
		}
	}

	handle = stateChangeResponse.Payload

	// if no timeout in seconds provided then set to default of 10
	if seconds == nil {
		s := 10
		seconds = &s
	}

	err = c.CommitContainerHandle(ctx, handle, vc.ContainerID, int32(*seconds))
	if err != nil {
		if errors.IsNotFoundError(err) {
			cache.ContainerCache().DeleteContainer(vc.ContainerID)
		}
		return err
	}

	return nil
}

// UnbindContainerFromNetwork unbinds a container from the networks that it connects to
func (c *VicContainerProxy) UnbindContainerFromNetwork(ctx context.Context, vc *viccontainer.VicContainer, handle string) (string, error) {
	op := trace.FromContext(ctx, "UnbindContainerFromNetwork: %s", vc.ContainerID)
	defer trace.End(trace.Begin(vc.ContainerID, op))
	opID := op.ID()

	if c.client == nil {
		return "", errors.NillPortlayerClientError("ContainerProxy")
	}

	unbindParams := scopes.NewUnbindContainerParamsWithContext(ctx).WithOpID(&opID).WithHandle(handle)
	ub, err := c.client.Scopes.UnbindContainer(unbindParams)
	if err != nil {
		switch err := err.(type) {
		case *scopes.UnbindContainerNotFound:
			// ignore error
			op.Warnf("Container %s not found by network unbind", vc.ContainerID)
		case *scopes.UnbindContainerInternalServerError:
			return "", errors.InternalServerError(err.Payload.Message)
		default:
			return "", errors.InternalServerError(err.Error())
		}
	}

	return ub.Payload.Handle, nil
}

// State returns container state
func (c *VicContainerProxy) State(ctx context.Context, vc *viccontainer.VicContainer) (*types.ContainerState, error) {
	op := trace.FromContext(ctx, "State: %s", vc.ContainerID)
	defer trace.End(trace.Begin(vc.ContainerID, op))
	opID := op.ID()

	if c.client == nil {
		return nil, errors.NillPortlayerClientError("ContainerProxy")
	}

	results, err := c.client.Containers.GetContainerInfo(containers.NewGetContainerInfoParamsWithContext(ctx).
		WithOpID(&opID).
		WithID(vc.ContainerID))
	if err != nil {
		switch err := err.(type) {
		case *containers.GetContainerInfoNotFound:
			return nil, errors.NotFoundError(vc.Name)
		case *containers.GetContainerInfoInternalServerError:
			return nil, errors.InternalServerError(err.Payload.Message)
		default:
			return nil, errors.InternalServerError(fmt.Sprintf("Unknown error from the interaction port layer: %s", err))
		}
	}

	inspectJSON, err := ContainerInfoToDockerContainerInspect(vc, results.Payload, c.portlayerName)
	if err != nil {
		return nil, err
	}
	return inspectJSON.State, nil
}

// GetStateFromHandle takes a handle and returns the state of the container based on that handle. Also returns handle that comes back with the response.
func (c *VicContainerProxy) GetStateFromHandle(op trace.Operation, handle string) (string, string, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("handle(%s)", handle), op))
	opID := op.ID()

	if c.client == nil {
		return "", "", errors.NillPortlayerClientError("ContainerProxy")
	}

	params := containers.NewGetStateParams().WithOpID(&opID).WithHandle(handle)
	resp, err := c.client.Containers.GetState(params)
	if err != nil {
		switch err := err.(type) {
		case *containers.GetStateNotFound:
			return handle, "", errors.NotFoundError(err.Payload.Message)
		default:
			return handle, "", errors.InternalServerError(err.Error())
		}
	}

	return resp.Payload.Handle, resp.Payload.State, nil
}

// ExitCode returns container exitCode
func (c *VicContainerProxy) ExitCode(ctx context.Context, vc *viccontainer.VicContainer) (string, error) {
	op := trace.FromContext(ctx, "ExitCode: %s", vc.ContainerID)
	defer trace.End(trace.Begin(vc.ContainerID, op))
	opID := op.ID()

	if c.client == nil {
		return "", errors.NillPortlayerClientError("ContainerProxy")
	}

	results, err := c.client.Containers.GetContainerInfo(containers.NewGetContainerInfoParamsWithContext(ctx).
		WithOpID(&opID).
		WithID(vc.ContainerID))

	if err != nil {
		switch err := err.(type) {
		case *containers.GetContainerInfoNotFound:
			return "", errors.NotFoundError(vc.Name)
		case *containers.GetContainerInfoInternalServerError:
			return "", errors.InternalServerError(err.Payload.Message)
		default:
			return "", errors.InternalServerError(fmt.Sprintf("Unknown error from the interaction port layer: %s", err))
		}
	}
	// get the container state
	dockerState := convert.State(results.Payload)
	if dockerState == nil {
		return "", errors.InternalServerError("Unable to determine container state")
	}

	return strconv.Itoa(dockerState.ExitCode), nil
}

func (c *VicContainerProxy) Wait(ctx context.Context, vc *viccontainer.VicContainer, timeout time.Duration) (
	*types.ContainerState, error) {
	op := trace.FromContext(ctx, "Wait: %s", vc.ContainerID)
	defer trace.End(trace.Begin(vc.ContainerID, op))
	opID := op.ID()

	defer trace.End(trace.Begin(vc.ContainerID))

	if vc == nil {
		return nil, errors.InternalServerError("Wait bad arguments")
	}

	// Get an API client to the portlayer
	if c.client == nil {
		return nil, errors.NillPortlayerClientError("ContainerProxy")
	}

	params := containers.NewContainerWaitParamsWithContext(ctx).
		WithOpID(&opID).
		WithTimeout(int64(timeout.Seconds())).
		WithID(vc.ContainerID)

	results, err := c.client.Containers.ContainerWait(params)
	if err != nil {
		switch err := err.(type) {
		case *containers.ContainerWaitNotFound:
			// since the container wasn't found on the PL lets remove from the local
			// cache
			cache.ContainerCache().DeleteContainer(vc.ContainerID)
			return nil, errors.NotFoundError(vc.ContainerID)
		case *containers.ContainerWaitInternalServerError:
			return nil, errors.InternalServerError(err.Payload.Message)
		default:
			return nil, errors.InternalServerError(err.Error())
		}
	}

	if results == nil || results.Payload == nil {
		return nil, errors.InternalServerError("Unexpected swagger error")
	}

	dockerState := convert.State(results.Payload)
	if dockerState == nil {
		return nil, errors.InternalServerError("Unable to determine container state")
	}
	return dockerState, nil
}

func (c *VicContainerProxy) Signal(ctx context.Context, vc *viccontainer.VicContainer, sig uint64) error {
	op := trace.FromContext(ctx, "Signal: %s", vc.ContainerID)
	defer trace.End(trace.Begin(vc.ContainerID, op))
	opID := op.ID()

	if vc == nil {
		return errors.InternalServerError("Signal bad arguments")
	}

	if c.client == nil {
		return errors.NillPortlayerClientError("ContainerProxy")
	}

	if state, err := c.State(ctx, vc); !state.Running && err == nil {
		return fmt.Errorf("%s is not running", vc.ContainerID)
	}

	// If Docker CLI sends sig == 0, we use sigkill
	if sig == 0 {
		sig = uint64(syscall.SIGKILL)
	}
	params := containers.NewContainerSignalParamsWithContext(ctx).
		WithOpID(&opID).
		WithID(vc.ContainerID).
		WithSignal(int64(sig))

	if _, err := c.client.Containers.ContainerSignal(params); err != nil {
		switch err := err.(type) {
		case *containers.ContainerSignalNotFound:
			return errors.NotFoundError(vc.ContainerID)
		case *containers.ContainerSignalInternalServerError:
			return errors.InternalServerError(err.Payload.Message)
		default:
			return errors.InternalServerError(err.Error())
		}
	}

	if state, err := c.State(ctx, vc); !state.Running && err == nil {
		// unmap ports
		if err = network.UnmapPorts(vc.ContainerID, vc); err != nil {
			return err
		}
	}

	return nil
}

func (c *VicContainerProxy) Resize(ctx context.Context, id string, height, width int32) error {
	op := trace.FromContext(ctx, "Resize: %s", id)
	defer trace.End(trace.Begin(id, op))
	opID := op.ID()

	defer trace.End(trace.Begin(id))

	if c.client == nil {
		return errors.NillPortlayerClientError("ContainerProxy")
	}

	plResizeParam := interaction.NewContainerResizeParamsWithContext(ctx).
		WithOpID(&opID).
		WithID(id).
		WithHeight(height).
		WithWidth(width)

	_, err := c.client.Interaction.ContainerResize(plResizeParam)
	if err != nil {
		if _, isa := err.(*interaction.ContainerResizeNotFound); isa {
			return errors.ContainerResourceNotFoundError(id, "interaction connection")
		}

		// If we get here, most likely something went wrong with the port layer API server
		return errors.InternalServerError(err.Error())
	}

	return nil
}

// Rename calls the portlayer's RenameContainerHandler to update the container name in the handle,
// and then commit the new name to vSphere
func (c *VicContainerProxy) Rename(ctx context.Context, vc *viccontainer.VicContainer, newName string) error {
	op := trace.FromContext(ctx, "Rename: %s", vc.ContainerID)
	defer trace.End(trace.Begin(vc.ContainerID, op))
	opID := op.ID()

	//retrieve client to portlayer
	handle, err := c.Handle(context.TODO(), vc.ContainerID, vc.Name)
	if err != nil {
		return err
	}

	if c.client == nil {
		return errors.NillPortlayerClientError("ContainerProxy")
	}

	// Call the rename functionality in the portlayer.
	renameParams := containers.NewContainerRenameParamsWithContext(ctx).
		WithOpID(&opID).
		WithName(newName).
		WithHandle(handle)

	result, err := c.client.Containers.ContainerRename(renameParams)
	if err != nil {
		switch err := err.(type) {
		// Here we don't check the portlayer error type for *containers.ContainerRenameConflict since
		// (1) we already check that in persona cache for ConflictError and
		// (2) the container name in portlayer cache will be updated when committing the handle in the next step
		case *containers.ContainerRenameNotFound:
			return errors.NotFoundError(vc.Name)
		default:
			return errors.InternalServerError(err.Error())
		}
	}

	h := result.Payload

	// commit handle
	_, err = c.client.Containers.Commit(containers.NewCommitParamsWithContext(ctx).WithHandle(h))
	if err != nil {
		switch err := err.(type) {
		case *containers.CommitNotFound:
			return errors.NotFoundError(err.Payload.Message)
		case *containers.CommitConflict:
			return errors.ConflictError(err.Payload.Message)
		default:
			return errors.InternalServerError(err.Error())
		}
	}

	return nil
}

// Remove calls the portlayer's ContainerRemove handler to remove the container and its
// anonymous volumes if the remove flag is set.
func (c *VicContainerProxy) Remove(ctx context.Context, vc *viccontainer.VicContainer, config *types.ContainerRmConfig) error {
	op := trace.FromContext(ctx, "Remove: %s", vc.ContainerID)
	defer trace.End(trace.Begin(vc.ContainerID, op))
	opID := op.ID()

	if c.client == nil {
		return errors.NillPortlayerClientError("ContainerProxy")
	}

	id := vc.ContainerID
	_, err := c.client.Containers.ContainerRemove(containers.NewContainerRemoveParamsWithContext(ctx).
		WithOpID(&opID).
		WithID(id))
	if err != nil {
		switch err := err.(type) {
		case *containers.ContainerRemoveNotFound:
			// Remove container from persona cache, but don't return error to the user.
			cache.ContainerCache().DeleteContainer(id)
			return nil
		case *containers.ContainerRemoveDefault:
			return errors.InternalServerError(err.Payload.Message)
		case *containers.ContainerRemoveConflict:
			return derr.NewRequestConflictError(fmt.Errorf("You cannot remove a running container. Stop the container before attempting removal or use -f"))
		case *containers.ContainerRemoveInternalServerError:
			if err.Payload == nil || err.Payload.Message == "" {
				return errors.InternalServerError(err.Error())
			}
			return errors.InternalServerError(err.Payload.Message)
		default:
			return errors.InternalServerError(err.Error())
		}
	}

	// Once the container is removed, remove anonymous volumes (vc.Config.Volumes) if
	// the remove flag is set.
	if config.RemoveVolume && len(vc.Config.Volumes) > 0 {
		RemoveAnonContainerVols(ctx, c.client, id, vc)
	}

	return nil
}

//----------
// Utility Functions
//----------

func dockerContainerCreateParamsToTask(ctx context.Context, id, layerID string, cc types.ContainerCreateConfig) *tasks.JoinParams {
	config := &models.TaskJoinConfig{}

	var path string
	var args []string

	// we explicitly specify the ID for the primary task so that it's the same as the containerID
	config.ID = id

	// Set the filesystem namespace this task expects to run in
	config.Namespace = layerID

	// Expand cmd into entrypoint and args
	cmd := strslice.StrSlice(cc.Config.Cmd)
	if len(cc.Config.Entrypoint) != 0 {
		path, args = cc.Config.Entrypoint[0], append(cc.Config.Entrypoint[1:], cmd...)
	} else {
		path, args = cmd[0], cmd[1:]
	}

	// copy the path
	config.Path = path

	// copy the args
	config.Args = make([]string, len(args))
	copy(config.Args, args)

	// copy the env array
	config.Env = make([]string, len(cc.Config.Env))
	copy(config.Env, cc.Config.Env)

	// working dir
	config.WorkingDir = cc.Config.WorkingDir

	// user
	config.User = cc.Config.User

	// attach.  Always set to true otherwise we cannot attach later.
	// this tells portlayer container is attachable.
	config.Attach = true

	// openstdin
	config.OpenStdin = cc.Config.OpenStdin

	// tty
	config.Tty = cc.Config.Tty

	// container stop signal
	config.StopSignal = cc.Config.StopSignal

	log.Debugf("dockerContainerCreateParamsToTask = %+v", config)

	return tasks.NewJoinParamsWithContext(ctx).WithConfig(config)
}

func dockerContainerCreateParamsToPortlayer(ctx context.Context, cc types.ContainerCreateConfig, vc *viccontainer.VicContainer, imageStore string) *containers.CreateParams {
	config := &models.ContainerCreateConfig{}

	config.NumCpus = cc.HostConfig.CPUCount
	config.MemoryMB = cc.HostConfig.Memory

	// Layer/vmdk to use
	config.Layer = vc.LayerID

	// Image ID
	config.Image = vc.ImageID

	// Repo Requested
	config.RepoName = cc.Config.Image

	//copy friendly name
	config.Name = cc.Name

	// image store
	config.ImageStore = &models.ImageStore{Name: imageStore}

	// network
	config.NetworkDisabled = cc.Config.NetworkDisabled

	// Stuff the Docker labels into VIC container annotations
	if len(cc.Config.Labels) > 0 {
		convert.SetContainerAnnotation(config, convert.AnnotationKeyLabels, cc.Config.Labels)
	}
	// if autoremove then add to annotation
	if cc.HostConfig.AutoRemove {
		convert.SetContainerAnnotation(config, convert.AnnotationKeyAutoRemove, cc.HostConfig.AutoRemove)
	}

	// hostname
	config.Hostname = cc.Config.Hostname
	// domainname - https://github.com/moby/moby/issues/27067
	config.Domainname = cc.Config.Domainname

	log.Debugf("dockerContainerCreateParamsToPortlayer = %+v", config)

	return containers.NewCreateParamsWithContext(ctx).WithCreateConfig(config)
}

func toModelsNetworkConfig(cc types.ContainerCreateConfig) *models.NetworkConfig {
	if cc.Config.NetworkDisabled {
		return nil
	}

	nc := &models.NetworkConfig{
		NetworkName: cc.HostConfig.NetworkMode.NetworkName(),
	}

	// Docker supports link for bridge network and user defined network, we should handle that
	if len(cc.HostConfig.Links) > 0 {
		nc.Aliases = append(nc.Aliases, cc.HostConfig.Links...)
	}

	if cc.NetworkingConfig != nil {
		log.Debugf("EndpointsConfig: %#v", cc.NetworkingConfig)

		es, ok := cc.NetworkingConfig.EndpointsConfig[nc.NetworkName]
		if ok {
			if es.IPAMConfig != nil {
				nc.Address = es.IPAMConfig.IPv4Address
			}

			// Pass Links and Aliases to PL
			nc.Aliases = append(nc.Aliases, epoint.Alias(es)...)
		}
	}

	for p := range cc.HostConfig.PortBindings {
		log.Infof("*** toModelsNetworkConfig - portbinding = %#v", p)
		nc.Ports = append(nc.Ports, fromPortbinding(p, cc.HostConfig.PortBindings[p])...)
	}

	return nc
}

// fromPortbinding translate Port/PortBinding pair to string array with format "hostPort:containerPort/protocol" or
// "containerPort/protocol" if hostPort is empty
// HostIP is ignored here, cause VCH ip address might change. Will query back real interface address in docker ps
func fromPortbinding(port nat.Port, binding []nat.PortBinding) []string {
	var portMappings []string
	if len(binding) == 0 {
		portMappings = append(portMappings, string(port))
		return portMappings
	}

	proto, privatePort := nat.SplitProtoPort(string(port))
	for _, bind := range binding {
		var portMap string
		if bind.HostPort != "" {
			portMap = fmt.Sprintf("%s:%s/%s", bind.HostPort, privatePort, proto)
		} else {
			portMap = string(port)
		}
		portMappings = append(portMappings, portMap)
	}
	return portMappings
}

//-------------------------------------
// Inspect Utility Functions
//-------------------------------------

// ContainerInfoToDockerContainerInspect takes a ContainerInfo swagger-based struct
// returned from VIC's port layer and creates an engine-api based container inspect struct.
// There maybe other asset gathering if ContainerInfo does not have all the information
func ContainerInfoToDockerContainerInspect(vc *viccontainer.VicContainer, info *models.ContainerInfo, portlayerName string) (*types.ContainerJSON, error) {
	if vc == nil || info == nil || info.ContainerConfig == nil {
		return nil, errors.NotFoundError(fmt.Sprintf("No such container: %s", vc.ContainerID))
	}
	// get the docker state
	containerState := convert.State(info)

	inspectJSON := &types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State:           containerState,
			ResolvConfPath:  "",
			HostnamePath:    "",
			HostsPath:       "",
			Driver:          portlayerName,
			MountLabel:      "",
			ProcessLabel:    "",
			AppArmorProfile: "",
			ExecIDs:         vc.List(),
			HostConfig:      hostConfigFromContainerInfo(vc, info, portlayerName),
			GraphDriver:     types.GraphDriverData{Name: portlayerName},
			SizeRw:          nil,
			SizeRootFs:      nil,
		},
		Mounts:          MountsFromContainer(vc),
		Config:          containerConfigFromContainerInfo(vc, info),
		NetworkSettings: networkFromContainerInfo(vc, info),
	}

	if inspectJSON.NetworkSettings != nil {
		log.Debugf("Docker inspect - network settings = %#v", inspectJSON.NetworkSettings)
	} else {
		log.Debug("Docker inspect - network settings = nil")
	}

	if info.ProcessConfig != nil {
		inspectJSON.Path = info.ProcessConfig.ExecPath
		if len(info.ProcessConfig.ExecArgs) > 0 {
			// args[0] is the command and should not appear in the args list here
			inspectJSON.Args = info.ProcessConfig.ExecArgs[1:]
		}
	}

	if info.ContainerConfig != nil {
		// set the status to the inspect expected values
		containerState.Status = filter.DockerState(info.ContainerConfig.State)

		// https://github.com/docker/docker/blob/master/container/state.go#L77
		if containerState.Status == ContainerStopped {
			containerState.Status = ContainerExited
		}

		inspectJSON.Image = info.ContainerConfig.ImageID
		inspectJSON.LogPath = info.ContainerConfig.LogPath
		inspectJSON.RestartCount = int(info.ContainerConfig.RestartCount)
		inspectJSON.ID = info.ContainerConfig.ContainerID
		inspectJSON.Created = time.Unix(0, info.ContainerConfig.CreateTime).Format(time.RFC3339Nano)
		if len(info.ContainerConfig.Names) > 0 {
			inspectJSON.Name = fmt.Sprintf("/%s", info.ContainerConfig.Names[0])
		}
	}

	return inspectJSON, nil
}

// hostConfigFromContainerInfo() gets the hostconfig that is passed to the backend during
// docker create and updates any needed info
func hostConfigFromContainerInfo(vc *viccontainer.VicContainer, info *models.ContainerInfo, portlayerName string) *container.HostConfig {
	if vc == nil || vc.HostConfig == nil || info == nil {
		return nil
	}

	// Create a copy of the created container's hostconfig.  This is passed in during
	// container create
	hostConfig := *vc.HostConfig

	// Resources don't really map well to VIC so we leave most of them empty. If we look
	// at the struct in engine-api/types/container/host_config.go, Microsoft added
	// additional attributes to the struct that are applicable to Windows containers.
	// If understanding VIC's host resources are desirable, we should go down this
	// same route.
	//
	// The values we fill out below is an abridged list of the original struct.
	resourceConfig := container.Resources{
	// Applicable to all platforms
	//			CPUShares int64 `json:"CpuShares"` // CPU shares (relative weight vs. other containers)
	//			Memory    int64 // Memory limit (in bytes)

	//			// Applicable to UNIX platforms
	//			DiskQuota            int64           // Disk limit (in bytes)
	}

	hostConfig.VolumeDriver = portlayerName
	hostConfig.Resources = resourceConfig
	hostConfig.DNS = make([]string, 0)

	if len(info.Endpoints) > 0 {
		for _, ep := range info.Endpoints {
			for _, dns := range ep.Nameservers {
				if dns != "" {
					hostConfig.DNS = append(hostConfig.DNS, dns)
				}
			}
		}

		hostConfig.NetworkMode = container.NetworkMode(info.Endpoints[0].Scope)
	}

	hostConfig.PortBindings = network.PortMapFromContainer(vc, info)

	// Set this to json-file to force the docker CLI to allow us to use docker logs
	hostConfig.LogConfig.Type = forceLogType

	// get the autoremove annotation from the container annotations
	convert.ContainerAnnotation(info.ContainerConfig.Annotations, convert.AnnotationKeyAutoRemove, &hostConfig.AutoRemove)

	return &hostConfig
}

// mountsFromContainer derives []types.MountPoint (used in inspect) from the cached container
// data.
func MountsFromContainer(vc *viccontainer.VicContainer) []types.MountPoint {
	if vc == nil {
		return nil
	}

	var mounts []types.MountPoint

	rawAnonVolumes := make([]string, 0, len(vc.Config.Volumes))
	for k := range vc.Config.Volumes {
		rawAnonVolumes = append(rawAnonVolumes, k)
	}

	volList, err := finalizeVolumeList(vc.HostConfig.Binds, rawAnonVolumes)
	if err != nil {
		return mounts
	}

	for _, vol := range volList {
		mountConfig := types.MountPoint{
			Type:        mount.TypeVolume,
			Driver:      engconstants.DefaultVolumeDriver,
			Name:        vol.ID,
			Source:      vol.ID,
			Destination: vol.Dest,
			RW:          false,
			Mode:        vol.Flags,
		}

		if strings.Contains(strings.ToLower(vol.Flags), "rw") {
			mountConfig.RW = true
		}
		mounts = append(mounts, mountConfig)
	}

	return mounts
}

// containerConfigFromContainerInfo() returns a container.Config that has attributes
// overridden at create or start time.  This is important.  This function is called
// to help build the Container Inspect struct.  That struct contains the original
// container config that is part of the image metadata AND the overridden container
// config.  The user can override these via the remote API or the docker CLI.
func containerConfigFromContainerInfo(vc *viccontainer.VicContainer, info *models.ContainerInfo) *container.Config {
	if vc == nil || vc.Config == nil || info == nil || info.ContainerConfig == nil || info.ProcessConfig == nil {
		return nil
	}

	// Copy the working copy of our container's config
	container := *vc.Config

	if info.ContainerConfig.ContainerID != "" {
		container.Hostname = stringid.TruncateID(info.ContainerConfig.ContainerID) // Hostname
	}
	if info.ContainerConfig.AttachStdin != nil {
		container.AttachStdin = *info.ContainerConfig.AttachStdin // Attach the standard input, makes possible user interaction
	}
	if info.ContainerConfig.AttachStdout != nil {
		container.AttachStdout = *info.ContainerConfig.AttachStdout // Attach the standard output
	}
	if info.ContainerConfig.AttachStderr != nil {
		container.AttachStderr = *info.ContainerConfig.AttachStderr // Attach the standard error
	}
	if info.ContainerConfig.Tty != nil {
		container.Tty = *info.ContainerConfig.Tty // Attach standard streams to a tty, including stdin if it is not closed.
	}
	if info.ContainerConfig.OpenStdin != nil {
		container.OpenStdin = *info.ContainerConfig.OpenStdin
	}
	// They are not coming from PL so set them to true unconditionally
	container.StdinOnce = true

	if info.ContainerConfig.RepoName != nil {
		container.Image = *info.ContainerConfig.RepoName // Name of the image as it was passed by the operator (eg. could be symbolic)
	}

	// Fill in information about the process
	if info.ProcessConfig.Env != nil {
		container.Env = info.ProcessConfig.Env // List of environment variable to set in the container
	}

	if info.ProcessConfig.WorkingDir != nil {
		container.WorkingDir = *info.ProcessConfig.WorkingDir // Current directory (PWD) in the command will be launched
	}

	container.User = info.ProcessConfig.User

	// Fill in information about the container network
	if info.Endpoints == nil {
		container.NetworkDisabled = true
	} else {
		container.NetworkDisabled = false
		container.MacAddress = ""
		container.ExposedPorts = vc.Config.ExposedPorts
	}

	// Get the original container config from the image's metadata in our image cache.
	var imageConfig *metadata.ImageConfig

	if info.ContainerConfig.LayerID != "" {
		// #nosec: Errors unhandled.
		imageConfig, _ = cache.ImageCache().Get(info.ContainerConfig.LayerID)
	}

	// Fill in the values with defaults from the original image's container config
	// structure
	if imageConfig != nil {
		container.StopSignal = imageConfig.ContainerConfig.StopSignal // Signal to stop a container

		container.OnBuild = imageConfig.ContainerConfig.OnBuild // ONBUILD metadata that were defined on the image Dockerfile
	}

	// Pull labels from the annotation
	convert.ContainerAnnotation(info.ContainerConfig.Annotations, convert.AnnotationKeyLabels, &container.Labels)
	return &container
}

func networkFromContainerInfo(vc *viccontainer.VicContainer, info *models.ContainerInfo) *types.NetworkSettings {
	networks := &types.NetworkSettings{
		NetworkSettingsBase: types.NetworkSettingsBase{
			Bridge:                 "",
			SandboxID:              "",
			HairpinMode:            false,
			LinkLocalIPv6Address:   "",
			LinkLocalIPv6PrefixLen: 0,
			Ports:                  network.PortMapFromContainer(vc, info),
			SandboxKey:             "",
			SecondaryIPAddresses:   nil,
			SecondaryIPv6Addresses: nil,
		},
		Networks: make(map[string]*dnetwork.EndpointSettings),
	}

	shortCID := vc.ContainerID[0:ShortIDLen]

	// Fill in as much info from the endpoint struct inside of the ContainerInfo.
	// The rest of the data must be obtained from the Scopes portlayer.
	for _, ep := range info.Endpoints {
		netEp := &dnetwork.EndpointSettings{
			IPAMConfig:          nil, //Get from Scope PL
			Links:               nil,
			Aliases:             nil,
			NetworkID:           "", //Get from Scope PL
			EndpointID:          ep.ID,
			Gateway:             ep.Gateway,
			IPAddress:           "",
			IPPrefixLen:         0,  //Get from Scope PL
			IPv6Gateway:         "", //Get from Scope PL
			GlobalIPv6Address:   "", //Get from Scope PL
			GlobalIPv6PrefixLen: 0,  //Get from Scope PL
			MacAddress:          "", //Container endpoints currently do not have mac addr yet
		}

		if ep.Address != "" {
			ip, ipnet, err := net.ParseCIDR(ep.Address)
			if err == nil {
				netEp.IPAddress = ip.String()
				netEp.IPPrefixLen, _ = ipnet.Mask.Size()
			}
		}

		if len(ep.Aliases) > 0 {
			netEp.Aliases = make([]string, len(ep.Aliases))
			found := false
			for i, alias := range ep.Aliases {
				netEp.Aliases[i] = alias
				if alias == shortCID {
					found = true
				}
			}

			if !found {
				netEp.Aliases = append(netEp.Aliases, vc.ContainerID[0:ShortIDLen])
			}
		}

		networks.Networks[ep.Scope] = netEp
	}

	return networks
}

// addExposedToPortMap ensures that exposed ports are all present in the port map.
// This means nil entries for any exposed ports that are not mapped.
// The portMap provided is modified and returned - the return value should always be used.
func addExposedToPortMap(config *container.Config, portMap nat.PortMap) nat.PortMap {
	if config == nil || len(config.ExposedPorts) == 0 {
		return portMap
	}

	if portMap == nil {
		portMap = make(nat.PortMap)
	}

	for p := range config.ExposedPorts {
		if _, ok := portMap[p]; ok {
			continue
		}

		portMap[p] = nil
	}

	return portMap
}

func ContainerInfoToVicContainer(info models.ContainerInfo, portlayerName string) *viccontainer.VicContainer {
	vc := viccontainer.NewVicContainer()

	if info.ContainerConfig.ContainerID != "" {
		vc.ContainerID = info.ContainerConfig.ContainerID
	}

	log.Debugf("Convert container info to vic container: %s", vc.ContainerID)

	if len(info.ContainerConfig.Names) > 0 {
		vc.Name = info.ContainerConfig.Names[0]
		log.Debugf("Container %q", vc.Name)
	}

	if info.ContainerConfig.LayerID != "" {
		vc.LayerID = info.ContainerConfig.LayerID
	}

	if info.ContainerConfig.ImageID != "" {
		vc.ImageID = info.ContainerConfig.ImageID
	}

	tempVC := viccontainer.NewVicContainer()
	tempVC.HostConfig = &container.HostConfig{}
	vc.Config = containerConfigFromContainerInfo(tempVC, &info)
	vc.HostConfig = hostConfigFromContainerInfo(tempVC, &info, portlayerName)

	// FIXME: duplicate Config.Volumes and HostConfig.Binds here for can not derive them from persisted value right now.
	// get volumes from volume config
	vc.Config.Volumes = make(map[string]struct{}, len(info.VolumeConfig))
	vc.HostConfig.Binds = []string{}
	for _, volume := range info.VolumeConfig {
		mount := getMountString(volume.Name, volume.MountPoint, volume.Flags[constants.Mode])
		vc.Config.Volumes[mount] = struct{}{}
		vc.HostConfig.Binds = append(vc.HostConfig.Binds, mount)
		log.Debugf("add volume mount %s to config.volumes and hostconfig.binds", mount)
	}

	vc.Config.Cmd = info.ProcessConfig.ExecArgs

	return vc
}
