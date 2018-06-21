package proxy

import (
	"fmt"
	"time"

	vicerrors "github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/containers"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/interaction"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/logging"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/scopes"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/storage"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/tasks"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/sys"

	"github.com/docker/docker/api/types/strslice"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/cache"
	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/constants"
)

type IsolationProxy interface {
	CreateHandle(op trace.Operation) (string, string, error)
	AddImageToHandle(op trace.Operation, handle, deltaID, layerID, imageID, imageName string) (string, error)
	CreateHandleTask(op trace.Operation, handle, id, layerID string, config IsolationContainerConfig) (string, error)
	AddHandleToScope(op trace.Operation, handle string, config IsolationContainerConfig) (string, error)
	AddInteractionToHandle(op trace.Operation, handle string) (string, error)
	AddLoggingToHandle(op trace.Operation, handle string) (string, error)
	CommitHandle(op trace.Operation, handle, containerID string, waitTime int32) error
	SetState(op trace.Operation, handle, name, state string) (string, error)

	BindScope(op trace.Operation, handle string, name string) (string, interface{}, error)
	UnbindScope(op trace.Operation, handle string, name string) (string, interface{}, error)

	Handle(op trace.Operation, id, name string) (string, error)
	Remove(op trace.Operation, id string, force bool) error

	State(op trace.Operation, id, name string) (string, error)
	EpAddresses(op trace.Operation, id, name string) ([]string, error)
}

type VicIsolationProxy struct {
	client        *client.PortLayer
	imageStore    ImageStore
	podCache      cache.PodCache
	portlayerAddr string
	hostUUID      string
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

func NewIsolationProxy(plClient *client.PortLayer, portlayerAddr string, hostUUID string, imageStore ImageStore, podCache cache.PodCache) IsolationProxy {
	if plClient == nil {
		return nil
	}

	return &VicIsolationProxy{
		client:        plClient,
		imageStore:    imageStore,
		podCache:      podCache,
		portlayerAddr: portlayerAddr,
		hostUUID:      hostUUID,
	}
}

// CreateHandle creates a "manifest" that will be used by Commit() to create the actual
// isolation vm.
//
// returns:
//	(container/pod id, handle, error)
func (v *VicIsolationProxy) CreateHandle(op trace.Operation) (string, string, error) {
	defer trace.End(trace.Begin("CreateHandle", op))

	if v.client == nil {
		return "", "", vicerrors.NillPortlayerClientError("IsolationProxy")
	}

	// Call the Exec port layer to create the container
	var err error
	var hostUUID string
	if v.hostUUID != "" {
		hostUUID = v.hostUUID
	} else {
		hostUUID, err = sys.UUID()
	}

	if err != nil {
		return "", "", vicerrors.InternalServerError("IsolationProxy.CreateContainerHandle got unexpected error getting VCH UUID")
	}

	plCreateParams := initIsolationConfig(op, "", constants.DummyRepoName, constants.DummyImage, constants.DummyLayerID, hostUUID)
	createResults, err := v.client.Containers.Create(plCreateParams)
	if err != nil {
		if _, ok := err.(*containers.CreateNotFound); ok {
			cerr := fmt.Errorf("No such image: %s", constants.DummyImage)
			op.Errorf("%s (%s)", cerr, err)
			return "", "", vicerrors.NotFoundError(cerr.Error())
		}

		// If we get here, most likely something went wrong with the port layer API server
		return "", "", vicerrors.InternalServerError(err.Error())
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
		return "", vicerrors.NillPortlayerClientError("IsolationProxy")
	}

	resp, err := v.client.Containers.Get(containers.NewGetParamsWithContext(op).WithID(id))
	if err != nil {
		switch err := err.(type) {
		case *containers.GetNotFound:
			return "", vicerrors.NotFoundError(name)
		case *containers.GetDefault:
			return "", vicerrors.InternalServerError(err.Payload.Message)
		default:
			return "", vicerrors.InternalServerError(err.Error())
		}
	}
	return resp.Payload, nil
}

func (v *VicIsolationProxy) AddImageToHandle(op trace.Operation, handle, deltaID, layerID, imageID, imageName string) (string, error) {
	defer trace.End(trace.Begin(handle, op))

	if v.client == nil {
		return "", vicerrors.InternalServerError("IsolationProxy.AddImageToContainer failed to get the portlayer client")
	}

	var err error
	var hostUUID string
	if v.hostUUID != "" {
		hostUUID = v.hostUUID
	} else {
		hostUUID, err = sys.UUID()
	}

	if err != nil {
		return "", vicerrors.InternalServerError("IsolationProxy.AddImageToContainer got unexpected error getting VCH UUID")
	}

	response, err := v.client.Storage.ImageJoin(storage.NewImageJoinParamsWithContext(op).WithStoreName(hostUUID).WithID(layerID).
		WithConfig(&models.ImageJoinConfig{
			Handle:   handle,
			DeltaID:  deltaID,
			ImageID:  imageID,
			RepoName: imageName,
		}))
	if err != nil {
		return "", vicerrors.InternalServerError(err.Error())
	}
	handle, ok := response.Payload.Handle.(string)
	if !ok {
		return "", vicerrors.InternalServerError(fmt.Sprintf("Type assertion failed for %#+v", handle))
	}

	return handle, nil
}

func (v *VicIsolationProxy) CreateHandleTask(op trace.Operation, handle, id, layerID string, config IsolationContainerConfig) (string, error) {
	defer trace.End(trace.Begin(handle, op))

	if v.client == nil {
		return "", vicerrors.InternalServerError("IsolationProxy.CreateContainerTask failed to create a portlayer client")
	}

	op.Debugf("CreateHandleTask - %#v", config)

	plTaskParams := IsolationContainerConfigToTask(op, id, layerID, config)
	plTaskParams.Config.Handle = handle

	op.Debugf("*** CreateContainerTask - params = %#v", *plTaskParams.Config)
	responseJoin, err := v.client.Tasks.Join(plTaskParams)
	if err != nil {
		op.Errorf("Unable to join primary task to container: %+v", err)
		return "", vicerrors.InternalServerError(err.Error())
	}

	handle, ok := responseJoin.Payload.Handle.(string)
	if !ok {
		return "", vicerrors.InternalServerError(fmt.Sprintf("Type assertion failed on handle from task join: %#+v", handle))
	}

	plBindParams := tasks.NewBindParamsWithContext(op).WithConfig(&models.TaskBindConfig{Handle: handle, ID: id})
	responseBind, err := v.client.Tasks.Bind(plBindParams)
	if err != nil {
		op.Errorf("Unable to bind primary task to container: %+v", err)
		return "", vicerrors.InternalServerError(err.Error())
	}

	handle, ok = responseBind.Payload.Handle.(string)
	if !ok {
		return "", vicerrors.InternalServerError(fmt.Sprintf("Type assertion failed on handle from task bind %#+v", handle))
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
		return "", vicerrors.NillPortlayerClientError("IsolationProxy")
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
			op.Errorf("IsolationProxy.AddContainerToScope: Scopes error: %s", err.Error())
			return handle, vicerrors.InternalServerError(err.Error())
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
		return "", vicerrors.NillPortlayerClientError("IsolationProxy")
	}

	response, err := v.client.Logging.LoggingJoin(logging.NewLoggingJoinParamsWithContext(op).
		WithConfig(&models.LoggingJoinConfig{
			Handle: handle,
		}))
	if err != nil {
		return "", vicerrors.InternalServerError(err.Error())
	}
	handle, ok := response.Payload.Handle.(string)
	if !ok {
		return "", vicerrors.InternalServerError(fmt.Sprintf("Type assertion failed for %#+v", handle))
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
		return "", vicerrors.NillPortlayerClientError("IsolationProxy")
	}

	response, err := v.client.Interaction.InteractionJoin(interaction.NewInteractionJoinParamsWithContext(op).
		WithConfig(&models.InteractionJoinConfig{
			Handle: handle,
		}))
	if err != nil {
		return "", vicerrors.InternalServerError(err.Error())
	}
	handle, ok := response.Payload.Handle.(string)
	if !ok {
		return "", vicerrors.InternalServerError(fmt.Sprintf("Type assertion failed for %#+v", handle))
	}

	return handle, nil
}

func (v *VicIsolationProxy) CommitHandle(op trace.Operation, handle, containerID string, waitTime int32) error {
	defer trace.End(trace.Begin(handle, op))

	if v.client == nil {
		return vicerrors.NillPortlayerClientError("IsolationProxy")
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
			return vicerrors.NotFoundError(containerID)
		case *containers.CommitConflict:
			return vicerrors.ConflictError(err.Error())
		case *containers.CommitDefault:
			return vicerrors.InternalServerError(err.Payload.Message)
		default:
			return vicerrors.InternalServerError(err.Error())
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
		return "", nil, vicerrors.NillPortlayerClientError("IsolationProxy")
	}

	bindParams := scopes.NewBindContainerParamsWithContext(op).WithHandle(handle)
	bindRes, err := v.client.Scopes.BindContainer(bindParams)
	if err != nil {
		switch err := err.(type) {
		case *scopes.BindContainerNotFound:
			return "", nil, vicerrors.NotFoundError(name)
		case *scopes.BindContainerInternalServerError:
			return "", nil, vicerrors.InternalServerError(err.Payload.Message)
		default:
			return "", nil, vicerrors.InternalServerError(err.Error())
		}
	}

	return bindRes.Payload.Handle, bindRes.Payload.Endpoints, nil
}

func (v *VicIsolationProxy) UnbindScope(op trace.Operation, handle string, name string) (string, interface{}, error) {
	defer trace.End(trace.Begin(handle, op))

	if v.client == nil {
		return "", nil, vicerrors.NillPortlayerClientError("IsolationProxy")
	}

	unbindParams := scopes.NewUnbindContainerParamsWithContext(op).WithHandle(handle)
	resp, err := v.client.Scopes.UnbindContainer(unbindParams)
	if err != nil {
		switch err := err.(type) {
		case *scopes.UnbindContainerNotFound:
			return "", nil, vicerrors.NotFoundError(name)
		case *scopes.UnbindContainerInternalServerError:
			return "", nil, vicerrors.InternalServerError(err.Payload.Message)
		default:
			return "", nil, vicerrors.InternalServerError(err.Error())
		}
	}

	return resp.Payload.Handle, resp.Payload.Endpoints, nil
}

// SetState adds the desire state of the isolation unit once the handle is commited.
//
//   returns handle string and error
func (v *VicIsolationProxy) SetState(op trace.Operation, handle, name, state string) (string, error) {
	defer trace.End(trace.Begin(handle, op))

	if v.client == nil {
		return "", vicerrors.NillPortlayerClientError("IsolationProxy")
	}

	resp, err := v.client.Containers.StateChange(containers.NewStateChangeParamsWithContext(op).WithHandle(handle).WithState(state))
	if err != nil {
		switch err := err.(type) {
		case *containers.StateChangeNotFound:
			return "", vicerrors.NotFoundError(name)
		case *containers.StateChangeDefault:
			return "", vicerrors.InternalServerError(err.Payload.Message)
		default:
			return "", vicerrors.InternalServerError(err.Error())
		}
	}

	return resp.Payload, nil
}

func (v *VicIsolationProxy) Remove(op trace.Operation, id string, force bool) error {
	defer trace.End(trace.Begin(id, op))

	if v.client == nil {
		return vicerrors.NillPortlayerClientError("IsolationProxy")
	}

	pForce := force
	params := containers.NewContainerRemoveParamsWithContext(op).
		WithID(id).
		WithForce(&pForce).
		WithTimeout(120 * time.Second)

	removeOK, err := v.client.Containers.ContainerRemove(params)
	op.Debugf("ContainerRemove returned %# +v", removeOK)
	return err
}

func (v *VicIsolationProxy) State(op trace.Operation, id, name string) (string, error) {
	defer trace.End(trace.Begin(id, op))

	payload, err := v.getInfo(op, id, name)
	if err != nil {
		return "", err
	}

	state := payload.ContainerConfig.State
	return state, nil
}

func (v *VicIsolationProxy) EpAddresses(op trace.Operation, id, name string) ([]string, error) {
	defer trace.End(trace.Begin(id, op))

	payload, err := v.getInfo(op, id, name)
	if err != nil {
		return nil, err
	}

	addresses := make([]string, 0)
	for _, ep := range payload.Endpoints {
		addresses = append(addresses, ep.Address)
	}

	return addresses, nil
}

// Private methods
func (v *VicIsolationProxy) getInfo(op trace.Operation, id, name string) (*models.ContainerInfo, error) {
	defer trace.End(trace.Begin(id, op))

	if v.client == nil {
		return nil, vicerrors.NillPortlayerClientError("IsolationProxy")
	}

	results, err := v.client.Containers.GetContainerInfo(containers.NewGetContainerInfoParamsWithContext(op).WithID(id))
	if err != nil {
		switch err := err.(type) {
		case *containers.GetContainerInfoNotFound:
			return nil, vicerrors.NotFoundError(name)
		case *containers.GetContainerInfoInternalServerError:
			return nil, vicerrors.InternalServerError(err.Payload.Message)
		default:
			return nil, vicerrors.InternalServerError(fmt.Sprintf("Unknown error from port layer: %s", err))
		}
	}

	return results.Payload, nil
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

	config.NumCpus = constants.DefaultCPUs
	config.MemoryMB = constants.DefaultMemory

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
	config.Hostname = constants.HostName
	//// domainname - https://github.com/moby/moby/issues/27067
	//config.Domainname = cc.Config.Domainname

	op.Debugf("dockerContainerCreateParamsToPortlayer = %+v", config)

	return containers.NewCreateParamsWithContext(op).WithCreateConfig(config)
}

func networkConfigFromIsolationConfig(config IsolationContainerConfig) *models.NetworkConfig {
	nc := &models.NetworkConfig{
		NetworkName: "default",
	}

	for key, val := range config.PortMap {
		nc.Ports = append(nc.Ports, fmt.Sprintf("%s:%s", val.HostPort, key))
	}

	return nc
}
