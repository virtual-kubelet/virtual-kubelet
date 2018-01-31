// Copyright 2018 VMware, Inc. All Rights Reserved.
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

import (
	"fmt"

	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/containers"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/interaction"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/logging"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/scopes"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/storage"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/tasks"

	"github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/pkg/trace"

	"github.com/docker/docker/api/types/strslice"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/cache"
	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/constants"
	"github.com/vmware/vic/pkg/vsphere/sys"
)

type IsolationProxy interface {
	CreateHandle(op trace.Operation) (string, string, error)
	AddImageToHandle(op trace.Operation, handle, deltaID, layerID, imageID, imageName string) (string, error)
	CreateHandleTask(op trace.Operation, handle, id, layerID string, config IsolationContainerConfig) (string, error)
	AddHandleToScope(op trace.Operation, handle string, config IsolationContainerConfig) (string, error)
	AddInteractionToHandle(op trace.Operation, handle string) (string, error)
	AddLoggingToHandle(op trace.Operation, handle string) (string, error)
	CommitHandle(op trace.Operation, handle, containerID string, waitTime int32) error

	BindScope(op trace.Operation, handle string, name string) (string, interface{}, error)
	UnbindScope(op trace.Operation, handle string, name string) (string, interface{}, error)

	Handle(op trace.Operation, id, name string) (string, error)
	SetState(op trace.Operation, handle, name, state string) (string, error)
}

type VicIsolationProxy struct {
	client        *client.PortLayer
	imageStore    ImageStore
	podCache      cache.PodCache
	portlayerAddr string
}

type PortBinding struct {
	HostIP   string
	HostPort string
}

type IsolationContainerConfig struct {
	ID        string
	ImageID   string
	LayerID   string
	ImageName string
	Name      string
	Namespace string

	Cmd        []string
	Path       string
	Entrypoint []string
	//Args       []string
	Env        []string
	WorkingDir string
	User       string
	StopSignal string

	Attach    bool
	StdinOnce bool
	OpenStdin bool
	Tty       bool

	CPUCount int64
	Memory   int64

	PortMap map[string]PortBinding
}

const (
	// DefaultCPUs - the default number of container VM CPUs
	DefaultCPUs   = 2
	DefaultMemory = 512

	DummyImage    = "f6e427c148a766d2d6c117d67359a0aa7d133b5bc05830a7ff6e8b64ff6b1d1d" //busybox
	DummyLayerID  = "02d3847f0b0fb7acd4419040cc53febf91cb112db2451d9b27a245dee5b227c0" //busybox
	DummyRepoName = "busybox"
)

func NewIsolationProxy(plClient *client.PortLayer, portlayerAddr string, imageStore ImageStore, podCache cache.PodCache) IsolationProxy {
	if plClient == nil {
		return nil
	}

	return &VicIsolationProxy{
		client:        plClient,
		imageStore:    imageStore,
		podCache:      podCache,
		portlayerAddr: portlayerAddr,
	}
}

func (v *VicIsolationProxy) CreateHandle(op trace.Operation) (string, string, error) {
	defer trace.End(trace.Begin("", op))

	if v.client == nil {
		return "", "", errors.NillPortlayerClientError("ContainerProxy")
	}

	// Call the Exec port layer to create the container
	var err error
	var host string
	if constants.RunningInVCH {
		host, err = sys.UUID()
	} else {
		host = constants.HostUUID
		err = nil
	}
	if err != nil {
		return "", "", errors.InternalServerError("ContainerProxy.CreateContainerHandle got unexpected error getting VCH UUID")
	}

	plCreateParams := initIsolationConfig(op, "", DummyRepoName, DummyImage, DummyLayerID, host)
	createResults, err := v.client.Containers.Create(plCreateParams)
	if err != nil {
		if _, ok := err.(*containers.CreateNotFound); ok {
			cerr := fmt.Errorf("No such image: %s", DummyImage)
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

// Handle retrieves a handle to a VIC container.  Handles should be treated as opaque strings.
//
// returns:
//	(handle string, error)
func (v *VicIsolationProxy) Handle(op trace.Operation, id, name string) (string, error) {
	if v.client == nil {
		return "", errors.NillPortlayerClientError("ContainerProxy")
	}

	resp, err := v.client.Containers.Get(containers.NewGetParamsWithContext(op).WithID(id))
	if err != nil {
		switch err := err.(type) {
		case *containers.GetNotFound:
			return "", errors.NotFoundError(name)
		case *containers.GetDefault:
			return "", errors.InternalServerError(err.Payload.Message)
		default:
			return "", errors.InternalServerError(err.Error())
		}
	}
	return resp.Payload, nil
}

func (v *VicIsolationProxy) AddImageToHandle(op trace.Operation, handle, deltaID, layerID, imageID, imageName string) (string, error) {
	defer trace.End(trace.Begin(handle, op))

	if v.client == nil {
		return "", errors.InternalServerError("ContainerProxy.AddImageToContainer failed to get the portlayer client")
	}

	var err error
	var host string
	if constants.RunningInVCH {
		host, err = sys.UUID()
	} else {
		host = constants.HostUUID
		err = nil
	}
	if err != nil {
		return "", errors.InternalServerError("ContainerProxy.AddImageToContainer got unexpected error getting VCH UUID")
	}

	response, err := v.client.Storage.ImageJoin(storage.NewImageJoinParamsWithContext(op).WithStoreName(host).WithID(layerID).
		WithConfig(&models.ImageJoinConfig{
			Handle:   handle,
			DeltaID:  deltaID,
			ImageID:  imageID,
			RepoName: imageName,
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

func (v *VicIsolationProxy) CreateHandleTask(op trace.Operation, handle, id, layerID string, config IsolationContainerConfig) (string, error) {
	defer trace.End(trace.Begin(handle, op))

	if v.client == nil {
		return "", errors.InternalServerError("IsolationProxy.CreateContainerTask failed to create a portlayer client")
	}

	op.Infof("*** CreateHandleTask - %#v", config)

	plTaskParams := IsolationContainerConfigToTask(op, id, layerID, config)
	plTaskParams.Config.Handle = handle

	op.Infof("*** CreateContainerTask - params = %#v", *plTaskParams.Config)
	responseJoin, err := v.client.Tasks.Join(plTaskParams)
	if err != nil {
		op.Errorf("Unable to join primary task to container: %+v", err)
		return "", errors.InternalServerError(err.Error())
	}

	handle, ok := responseJoin.Payload.Handle.(string)
	if !ok {
		return "", errors.InternalServerError(fmt.Sprintf("Type assertion failed on handle from task join: %#+v", handle))
	}

	plBindParams := tasks.NewBindParamsWithContext(op).WithConfig(&models.TaskBindConfig{Handle: handle, ID: id})
	responseBind, err := v.client.Tasks.Bind(plBindParams)
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

// AddHandleToScope adds a container, referenced by handle, to a scope.
// If an error is return, the returned handle should not be used.
//
// returns:
//	modified handle
func (v *VicIsolationProxy) AddHandleToScope(op trace.Operation, handle string, config IsolationContainerConfig) (string, error) {
	defer trace.End(trace.Begin(handle, op))

	if v.client == nil {
		return "", errors.NillPortlayerClientError("IsolationProxy")
	}

	// configure network
	netConf := networkConfigFromIsolationConfig(config)
	if netConf != nil {
		addContRes, err := v.client.Scopes.AddContainer(scopes.NewAddContainerParamsWithContext(op).
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
			if _, err2 := v.client.Scopes.RemoveContainer(scopes.NewRemoveContainerParamsWithContext(op).WithHandle(handle).WithScope(netConf.NetworkName)); err2 != nil {
				op.Warnf("could not roll back container add: %s", err2)
			}
		}()

		handle = addContRes.Payload
	}

	return handle, nil
}

// AddLoggingToHandle adds logging capability to the isolation vm, referenced by handle.
// If an error is return, the returned handle should not be used.
//
// returns:
//	modified handle
func (v *VicIsolationProxy) AddLoggingToHandle(op trace.Operation, handle string) (string, error) {
	defer trace.End(trace.Begin(handle, op))

	if v.client == nil {
		return "", errors.NillPortlayerClientError("IsolationProxy")
	}

	response, err := v.client.Logging.LoggingJoin(logging.NewLoggingJoinParamsWithContext(op).
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
func (v *VicIsolationProxy) AddInteractionToHandle(op trace.Operation, handle string) (string, error) {
	defer trace.End(trace.Begin(handle, op))

	if v.client == nil {
		return "", errors.NillPortlayerClientError("IsolationProxy")
	}

	response, err := v.client.Interaction.InteractionJoin(interaction.NewInteractionJoinParamsWithContext(op).
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

func (v *VicIsolationProxy) CommitHandle(op trace.Operation, handle, containerID string, waitTime int32) error {
	defer trace.End(trace.Begin(handle, op))

	if v.client == nil {
		return errors.NillPortlayerClientError("IsolationProxy")
	}

	var commitParams *containers.CommitParams
	if waitTime > 0 {
		commitParams = containers.NewCommitParamsWithContext(op).WithHandle(handle).WithWaitTime(&waitTime)
	} else {
		commitParams = containers.NewCommitParamsWithContext(op).WithHandle(handle)
	}

	_, err := v.client.Containers.Commit(commitParams)
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

//TODO:  I don't think this function should be in here.
// BindNetwork binds the handle to the scope and returns endpoints.  Caller of the function does not need
// to interpret the return value.  In the event the caller want to unbind,
func (v *VicIsolationProxy) BindScope(op trace.Operation, handle string, name string) (string, interface{}, error) {
	defer trace.End(trace.Begin(handle, op))

	if v.client == nil {
		return "", nil, errors.NillPortlayerClientError("IsolationProxy")
	}

	bindParams := scopes.NewBindContainerParamsWithContext(op).WithHandle(handle)
	bindRes, err := v.client.Scopes.BindContainer(bindParams)
	if err != nil {
		switch err := err.(type) {
		case *scopes.BindContainerNotFound:
			return "", nil, errors.NotFoundError(name)
		case *scopes.BindContainerInternalServerError:
			return "", nil, errors.InternalServerError(err.Payload.Message)
		default:
			return "", nil, errors.InternalServerError(err.Error())
		}
	}

	return bindRes.Payload.Handle, bindRes.Payload.Endpoints, nil
}

func (v *VicIsolationProxy) UnbindScope(op trace.Operation, handle string, name string) (string, interface{}, error) {
	defer trace.End(trace.Begin(handle, op))

	if v.client == nil {
		return "", nil, errors.NillPortlayerClientError("IsolationProxy")
	}

	unbindParams := scopes.NewUnbindContainerParamsWithContext(op).WithHandle(handle)
	resp, err := v.client.Scopes.UnbindContainer(unbindParams)
	if err != nil {
		switch err := err.(type) {
		case *scopes.UnbindContainerNotFound:
			return "", nil, errors.NotFoundError(name)
		case *scopes.UnbindContainerInternalServerError:
			return "", nil, errors.InternalServerError(err.Payload.Message)
		default:
			return "", nil, errors.InternalServerError(err.Error())
		}
	}

	return resp.Payload.Handle, resp.Payload.Endpoints, nil
}

// SetState adds the desire state of the isolation unit once the handle is commited.
//
//   returns handle string and error
func (v *VicIsolationProxy) SetState(op trace.Operation, handle, name, state string) (string, error) {
	defer trace.End(trace.Begin("name", op))

	if v.client == nil {
		return "", errors.NillPortlayerClientError("IsolationProxy")
	}

	resp, err := v.client.Containers.StateChange(containers.NewStateChangeParamsWithContext(op).WithHandle(handle).WithState(state))
	if err != nil {
		switch err := err.(type) {
		case *containers.StateChangeNotFound:
			return "", errors.NotFoundError(name)
		case *containers.StateChangeDefault:
			return "", errors.InternalServerError(err.Payload.Message)
		default:
			return "", errors.InternalServerError(err.Error())
		}
	}

	return resp.Payload, nil
}

//------------------------------------
// Utility Functions
//------------------------------------

// Convert isolation container config to portlayer task param
func IsolationContainerConfigToTask(op trace.Operation, id, layerID string, ic IsolationContainerConfig) *tasks.JoinParams {
	config := &models.TaskJoinConfig{}

	var path string
	var args []string

	// we explicitly specify the ID for the primary task so that it's the same as the containerID
	config.ID = id

	// Set the filesystem namespace this task expects to run in
	config.Namespace = layerID

	// Expand cmd into entrypoint and args
	cmd := strslice.StrSlice(ic.Cmd)
	if len(ic.Entrypoint) != 0 {
		path, args = ic.Entrypoint[0], append(ic.Entrypoint[1:], cmd...)
	} else {
		path, args = cmd[0], cmd[1:]
	}

	// copy the path
	config.Path = path

	// copy the args
	config.Args = make([]string, len(args))
	copy(config.Args, args)

	// copy the env array
	config.Env = make([]string, len(ic.Env))
	copy(config.Env, ic.Env)

	// working dir
	config.WorkingDir = ic.WorkingDir

	// user
	config.User = ic.User

	// attach.  Always set to true otherwise we cannot attach later.
	// this tells portlayer container is attachable.
	config.Attach = true

	// openstdin
	config.OpenStdin = ic.OpenStdin

	// tty
	config.Tty = ic.Tty

	// container stop signal
	config.StopSignal = ic.StopSignal

	op.Debugf("dockerContainerCreateParamsToTask = %+v", config)

	return tasks.NewJoinParamsWithContext(op).WithConfig(config)
}

// initIsolationConfig returns a default config used to create the isolation unit handle
func initIsolationConfig(op trace.Operation, name, repoName, imageID, layerID, imageStore string) *containers.CreateParams {
	config := &models.ContainerCreateConfig{}

	config.NumCpus = DefaultCPUs
	config.MemoryMB = DefaultMemory

	// Layer/vmdk to use
	config.Layer = layerID

	// Image ID
	config.Image = imageID

	// Repo Requested
	config.RepoName = repoName

	//copy friendly name
	config.Name = name

	// image store
	config.ImageStore = &models.ImageStore{Name: imageStore}

	// network
	config.NetworkDisabled = true

	// hostname
	config.Hostname = "test-kubelet"
	//// domainname - https://github.com/moby/moby/issues/27067
	//config.Domainname = cc.Config.Domainname

	op.Debugf("dockerContainerCreateParamsToPortlayer = %+v", config)

	return containers.NewCreateParamsWithContext(op).WithCreateConfig(config)
}

//HACK:  hard code for socat and nginx
func networkConfigFromIsolationConfig(config IsolationContainerConfig) *models.NetworkConfig {
	nc := &models.NetworkConfig{
		NetworkName: "default",
	}

	for key, val := range config.PortMap {
		nc.Ports = append(nc.Ports, fmt.Sprintf("%s:%s", val.HostPort, key))
	}

	return nc
}
