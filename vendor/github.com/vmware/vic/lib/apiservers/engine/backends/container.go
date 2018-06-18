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

package backends

import (
	"archive/tar"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/api/errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	containertypes "github.com/docker/docker/api/types/container"
	eventtypes "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dnetwork "github.com/docker/docker/api/types/network"
	timetypes "github.com/docker/docker/api/types/time"
	docker "github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/utils"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	viccontainer "github.com/vmware/vic/lib/apiservers/engine/backends/container"
	"github.com/vmware/vic/lib/apiservers/engine/backends/convert"
	"github.com/vmware/vic/lib/apiservers/engine/backends/filter"
	engerr "github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/engine/network"
	"github.com/vmware/vic/lib/apiservers/engine/proxy"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/containers"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/scopes"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/tasks"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/metadata"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/uid"
	"github.com/vmware/vic/pkg/vsphere/sys"
)

// valid filters as of docker commit 49bf474
var acceptedPsFilterTags = map[string]bool{
	"ancestor":  true,
	"before":    true,
	"exited":    true,
	"id":        true,
	"isolation": true,
	"label":     true,
	"name":      true,
	"status":    true,
	"health":    true,
	"since":     true,
	"volume":    true,
	"network":   true,
	"is-task":   true,
}

// currently not supported by vic
var unSupportedPsFilters = map[string]bool{
	"ancestor":  false,
	"health":    false,
	"isolation": false,
	"is-task":   false,
}

const (
	//bridgeIfaceName = "bridge"

	// MemoryAlignMB is the value to which container VM memory must align in order for hotadd to work
	MemoryAlignMB = 128
	// MemoryMinMB - the minimum allowable container memory size
	MemoryMinMB = 512
	// MemoryDefaultMB - the default container VM memory size
	MemoryDefaultMB = 2048
	// MinCPUs - the minimum number of allowable CPUs the container can use
	MinCPUs = 1
	// DefaultCPUs - the default number of container VM CPUs
	DefaultCPUs = 2
	// Default timeout to stop a container if not specified in container config
	DefaultStopTimeout = 10

	// maximum elapsed time for retry
	maxElapsedTime = 2 * time.Minute
)

var (
	defaultScope struct {
		sync.Mutex
		scope string
	}

	ctx = context.TODO()

	// allow mocking
	randomName = namesgenerator.GetRandomName
)

func init() {
	// seed the random number generator
	rand.Seed(time.Now().UTC().UnixNano())
}

// type and funcs to provide sorting by created date
type containerByCreated []*types.Container

func (r containerByCreated) Len() int           { return len(r) }
func (r containerByCreated) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r containerByCreated) Less(i, j int) bool { return r[i].Created < r[j].Created }

var containerEngine *ContainerBackend
var once sync.Once

// Container struct represents the Container
type ContainerBackend struct {
	containerProxy proxy.VicContainerProxy
	streamProxy    proxy.VicStreamProxy
	storageProxy   proxy.VicStorageProxy
}

// NewContainerBackend will create a new containerEngine or return the existing
func NewContainerBackend() *ContainerBackend {
	once.Do(func() {
		containerEngine = &ContainerBackend{
			containerProxy: proxy.NewContainerProxy(PortLayerClient(), PortLayerServer(), PortLayerName()),
			streamProxy:    proxy.NewStreamProxy(PortLayerClient()),
			storageProxy:   proxy.NewStorageProxy(PortLayerClient()),
		}
	})
	return containerEngine
}

const (
	defaultEnvPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
)

// docker's container.execBackend

func (c *ContainerBackend) TaskInspect(cid, cname, eid string) (*models.TaskInspectResponse, error) {
	// obtain a portlayer client
	client := PortLayerClient()

	handle, err := c.containerProxy.Handle(ctx, cid, cname)
	if err != nil {
		return nil, err
	}

	// inspect the Task to obtain ProcessConfig
	config := &models.TaskInspectConfig{
		Handle: handle,
		ID:     eid,
	}

	params := tasks.NewInspectParamsWithContext(ctx).WithConfig(config)
	resp, err := client.Tasks.Inspect(params)
	if err != nil {
		return nil, err
	}
	return resp.Payload, nil

}

func (c *ContainerBackend) TaskWaitToStart(cid, cname, eid string) error {
	// obtain a portlayer client
	client := PortLayerClient()

	handle, err := c.containerProxy.Handle(ctx, cid, cname)
	if err != nil {
		return err
	}

	// wait the Task to start
	config := &models.TaskWaitConfig{
		Handle: handle,
		ID:     eid,
	}

	params := tasks.NewWaitParamsWithContext(ctx).WithConfig(config)
	_, err = client.Tasks.Wait(params)
	if err != nil {
		switch err := err.(type) {
		case *tasks.WaitInternalServerError:
			return engerr.InternalServerError(err.Payload.Message)
		default:
			return engerr.InternalServerError(err.Error())
		}
	}

	return nil
}

// ContainerExecCreate sets up an exec in a running container.
func (c *ContainerBackend) ContainerExecCreate(name string, config *types.ExecConfig) (string, error) {
	op := trace.NewOperation(context.TODO(), "")
	defer trace.End(trace.Begin(fmt.Sprintf("%s: name=(%s)", op, name)))

	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return "", engerr.NotFoundError(name)
	}
	id := vc.ContainerID

	// Is it running?
	state, err := c.containerProxy.State(op, vc)
	if err != nil {
		return "", engerr.InternalServerError(err.Error())
	}

	if state.Restarting {
		return "", engerr.ConflictError(fmt.Sprintf("Container %s is restarting, wait until the container is running", id))
	}
	if !state.Running {
		return "", engerr.ConflictError(fmt.Sprintf("Container %s is not running", id))
	}

	op.Debugf("State checks succeeded for exec operation on cotnainer(%s)", id)
	handle, err := c.containerProxy.Handle(op, id, name)
	if err != nil {
		op.Error(err)
		return "", engerr.InternalServerError(err.Error())
	}

	// set up the environment
	config.Env = setEnvFromImageConfig(config.Tty, config.Env, vc.Config.Env)

	handleprime, eid, err := c.containerProxy.CreateExecTask(ctx, handle, config)
	if err != nil {
		op.Errorf("Failed to create exec task for container(%s) due to error(%s)", id, err)
		return "", engerr.InternalServerError(err.Error())
	}

	err = c.containerProxy.CommitContainerHandle(ctx, handleprime, id, 0)
	if err != nil {
		op.Errorf("Failed to commit exec handle for container(%s) due to error(%s)", id, err)
		return "", err
	}

	// associate newly created exec task with container
	cache.ContainerCache().AddExecToContainer(vc, eid)

	ec, err := c.TaskInspect(id, name, eid)
	if err != nil {
		switch err := err.(type) {
		case *tasks.InspectInternalServerError:
			op.Debugf("received an internal server error during task inspect: %s", err.Payload.Message)
			return "", engerr.InternalServerError(err.Payload.Message)
		case *tasks.InspectConflict:
			op.Debugf("received a conflict error during task inspect: %s", err.Payload.Message)
			return "", engerr.ConflictError(fmt.Sprintf("Cannot complete the operation, container %s has been powered off during execution", id))
		default:
			return "", engerr.InternalServerError(err.Error())
		}
	}

	// exec_create event
	event := "exec_create: " + ec.ProcessConfig.ExecPath + " " + strings.Join(ec.ProcessConfig.ExecArgs[1:], " ")
	actor := CreateContainerEventActorWithAttributes(vc, map[string]string{})
	EventService().Log(event, eventtypes.ContainerEventType, actor)

	return eid, nil
}

// ContainerExecInspect returns low-level information about the exec
// command. An error is returned if the exec cannot be found.
func (c *ContainerBackend) ContainerExecInspect(eid string) (*backend.ExecInspect, error) {
	op := trace.NewOperation(context.TODO(), "")
	defer trace.End(trace.Begin(fmt.Sprintf("opID=(%s) eid=(%s)", op, eid)))

	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainerFromExec(eid)
	if vc == nil {
		return nil, engerr.NotFoundError(eid)
	}
	id := vc.ContainerID
	name := vc.Name

	ec, err := c.TaskInspect(id, name, eid)
	if err != nil {
		switch err := err.(type) {
		case *tasks.InspectInternalServerError:
			op.Debugf("received an internal server error during task inspect: %s", err.Payload.Message)
			return nil, engerr.InternalServerError(err.Payload.Message)
		case *tasks.InspectConflict:
			op.Debugf("received a conflict error during task inspect: %s", err.Payload.Message)
			return nil, engerr.ConflictError(fmt.Sprintf("Cannot complete the operation, container %s has been powered off during execution", id))
		default:
			return nil, engerr.InternalServerError(err.Error())
		}
	}

	exit := int(ec.ExitCode)
	return &backend.ExecInspect{
		ID:       ec.ID,
		Running:  ec.Running,
		ExitCode: &exit,
		ProcessConfig: &backend.ExecProcessConfig{
			Tty:        ec.Tty,
			Entrypoint: ec.ProcessConfig.ExecPath,
			Arguments:  ec.ProcessConfig.ExecArgs,
			User:       ec.User,
		},
		OpenStdin:   ec.OpenStdin,
		OpenStdout:  ec.OpenStdout,
		OpenStderr:  ec.OpenStderr,
		ContainerID: vc.ContainerID,
		Pid:         int(ec.Pid),
	}, nil
}

// ContainerExecResize changes the size of the TTY of the process
// running in the exec with the given name to the given height and
// width.
func (c *ContainerBackend) ContainerExecResize(eid string, height, width int) error {
	defer trace.End(trace.Begin(eid))

	// Look up the container eid in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainerFromExec(eid)
	if vc == nil {
		return engerr.NotFoundError(eid)
	}

	// Call the port layer to resize
	plHeight := int32(height)
	plWidth := int32(width)

	var err error
	if err = c.containerProxy.Resize(ctx, eid, plHeight, plWidth); err == nil {
		actor := CreateContainerEventActorWithAttributes(vc, map[string]string{
			"height": fmt.Sprintf("%d", height),
			"width":  fmt.Sprintf("%d", width),
		})

		EventService().Log(containerResizeEvent, eventtypes.ContainerEventType, actor)
	}

	return err
}

// ContainerExecStart starts a previously set up exec instance. The
// std streams are set up.
func (c *ContainerBackend) ContainerExecStart(ctx context.Context, eid string, stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) error {
	op := trace.NewOperation(ctx, "")
	defer trace.End(trace.Begin(fmt.Sprintf("opID=(%s) eid=(%s)", op, eid)))

	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainerFromExec(eid)
	if vc == nil {
		return engerr.NotFoundError(eid)
	}
	id := vc.ContainerID
	name := vc.Name

	// grab the task details
	ec, err := c.TaskInspect(id, name, eid)
	if err != nil {
		switch err := err.(type) {
		case *tasks.InspectInternalServerError:
			op.Debugf("received an internal server error during task inspect: %s", err.Payload.Message)
			return engerr.InternalServerError(err.Payload.Message)
		case *tasks.InspectConflict:
			op.Debugf("received a conflict error during task inspect: %s", err.Payload.Message)
			return engerr.ConflictError(fmt.Sprintf("Cannot complete the operation, container %s has been powered off during execution", id))
		default:
			return engerr.InternalServerError(err.Error())
		}
	}

	handle, err := c.containerProxy.Handle(ctx, id, name)
	if err != nil {
		op.Errorf("Failed to obtain handle during exec start for container(%s) due to error: %s", id, err)
		return engerr.InternalServerError(err.Error())
	}

	bindconfig := &models.TaskBindConfig{
		Handle: handle,
		ID:     eid,
	}

	// obtain a portlayer client
	client := PortLayerClient()

	// call Bind with bindparams
	bindparams := tasks.NewBindParamsWithContext(ctx).WithConfig(bindconfig)
	resp, err := client.Tasks.Bind(bindparams)
	if err != nil {
		op.Errorf("Failed to bind parameters during exec start for container(%s) due to error: %s", id, err)
		return engerr.InternalServerError(err.Error())
	}
	handle = resp.Payload.Handle.(string)

	operation := func() error {
		// exec doesn't have separate attach path so we will decide whether we need interaction/runblocking or not
		attach := ec.OpenStdin || ec.OpenStdout || ec.OpenStderr
		if attach {
			handle, err = c.containerProxy.BindInteraction(ctx, handle, name, eid)
			if err != nil {
				op.Errorf("Failed to initiate interactivity during exec start for container(%s) due to error: %s", id, err)
				return err
			}
		}

		if err := c.containerProxy.CommitContainerHandle(ctx, handle, name, 0); err != nil {
			op.Errorf("Failed to commit handle for container(%s) due to error: %s", id, err)
			return err
		}

		// we need to be able to cancel it
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// we do not return an error here if this fails. TODO: Investigate what exactly happens on error here...
		go func() {
			defer trace.End(trace.Begin(eid))
			// wait property collector
			if err := c.TaskWaitToStart(id, name, eid); err != nil {
				op.Errorf("Task wait returned %s, canceling the context", err)

				// we can't return a proper error as we close the streams as soon as AttachStreams returns so we mimic Docker and write to stdout directly
				// https://github.com/docker/docker/blob/a039ca9affe5fa40c4e029d7aae399b26d433fe9/api/server/router/container/exec.go#L114
				if stdout != nil {
					stdout.Write([]byte(err.Error() + "\r\n"))
				}
				cancel()
			}
		}()

		// exec_start event
		event := "exec_start: " + ec.ProcessConfig.ExecPath + " " + strings.Join(ec.ProcessConfig.ExecArgs[1:], " ")

		actor := CreateContainerEventActorWithAttributes(vc, map[string]string{})
		EventService().Log(event, eventtypes.ContainerEventType, actor)

		// no need to attach for detached case
		if !attach {
			op.Debugf("Detached mode. Returning early.")
			return nil
		}
		EventService().Log(containerAttachEvent, eventtypes.ContainerEventType, actor)
		ca := &backend.ContainerAttachConfig{
			UseStdin:  ec.OpenStdin,
			UseStdout: ec.OpenStdout,
			UseStderr: ec.OpenStderr,
		}

		// set UseStderr to false for Tty case as we merge stdout and stderr
		if ec.Tty {
			ca.UseStderr = false
		}

		ac := &proxy.AttachConfig{
			ID: eid,
			ContainerAttachConfig: ca,
			UseTty:                ec.Tty,
			CloseStdin:            true,
		}

		err = c.streamProxy.AttachStreams(ctx, ac, stdin, stdout, stderr)
		if err != nil {
			if _, ok := err.(engerr.DetachError); ok {
				op.Infof("Detach detected, tearing down connection")

				// QUESTION: why are we returning DetachError? It doesn't seem like an error
				// fire detach event
				EventService().Log(containerDetachEvent, eventtypes.ContainerEventType, actor)

				// DON'T UNBIND FOR NOW, UNTIL/UNLESS REFERENCE COUNTING IS IN PLACE
				// This avoids cutting the communication channel for other sessions connected to this
				// container

				// FIXME: call UnbindInteraction/Commit
			}

			return err
		}
		return nil
	}
	if err := retry.Do(operation, engerr.IsConflictError); err != nil {
		op.Errorf("Failed to start Exec task for container(%s) due to error (%s)", id, err)
		return err
	}
	return nil
}

// ExecExists looks up the exec instance and returns a bool if it exists or not.
// It will also return the error produced by `getConfig`
func (c *ContainerBackend) ExecExists(eid string) (bool, error) {
	defer trace.End(trace.Begin(eid))

	vc := cache.ContainerCache().GetContainerFromExec(eid)
	if vc == nil {
		return false, engerr.NotFoundError(eid)
	}
	return true, nil
}

// ContainerCreate creates a container.
func (c *ContainerBackend) ContainerCreate(config types.ContainerCreateConfig) (containertypes.ContainerCreateCreatedBody, error) {
	defer trace.End(trace.Begin(""))

	var err error

	log.Infof("** createconfig = %#v", config)
	log.Infof("** container config = %#v", config.Config)

	// get the image from the cache
	image, err := cache.ImageCache().Get(config.Config.Image)
	if err != nil {
		// if no image found then error thrown and a pull
		// will be initiated by the docker client
		log.Errorf("ContainerCreate: image %s error: %s", config.Config.Image, err.Error())
		return containertypes.ContainerCreateCreatedBody{}, derr.NewRequestNotFoundError(err)
	}

	setCreateConfigOptions(config.Config, image.Config)

	log.Debugf("config.Config = %+v", config.Config)
	if err = validateCreateConfig(&config); err != nil {
		return containertypes.ContainerCreateCreatedBody{}, err
	}

	// Create a container representation in the personality server.  This representation
	// will be stored in the cache if create succeeds in the port layer.
	container, err := createInternalVicContainer(image, &config)
	if err != nil {
		return containertypes.ContainerCreateCreatedBody{}, err
	}

	// Reserve the container name to prevent duplicates during a parallel operation.
	if config.Name != "" {
		err := cache.ContainerCache().ReserveName(container, config.Name)
		if err != nil {
			return containertypes.ContainerCreateCreatedBody{}, derr.NewRequestConflictError(err)
		}
	} else {
		for i := 0; i < 5; i++ {
			generated := randomName(i)
			if cache.ContainerCache().ReserveName(container, generated) == nil {
				config.Name = generated
				break
			}
		}

		if config.Name == "" {
			return containertypes.ContainerCreateCreatedBody{}, derr.NewRequestConflictError(errors.New("attempted random names conflicted with existing containers"))
		}
	}

	// Create an actualized container in the VIC port layer
	id, err := c.containerCreate(container, config)
	if err != nil {
		cache.ContainerCache().ReleaseName(config.Name)
		return containertypes.ContainerCreateCreatedBody{}, err
	}

	// Container created ok, save the container id and save the config override from the API
	// caller and save this container internal representation in our personality server's cache
	copyConfigOverrides(container, config)
	container.ContainerID = id
	cache.ContainerCache().AddContainer(container)

	log.Debugf("Container create - name(%s), containerID(%s), config(%#v), host(%#v)",
		container.Name, container.ContainerID, container.Config, container.HostConfig)

	// Add create event
	actor := CreateContainerEventActorWithAttributes(container, map[string]string{})
	EventService().Log(containerCreateEvent, eventtypes.ContainerEventType, actor)

	return containertypes.ContainerCreateCreatedBody{ID: id}, nil
}

// createContainer() makes calls to the container proxy to actually create the backing
// VIC container.  All remoting code is in the proxy.
//
// returns:
//	(container id, error)
func (c *ContainerBackend) containerCreate(vc *viccontainer.VicContainer, config types.ContainerCreateConfig) (string, error) {
	defer trace.End(trace.Begin("Container.containerCreate"))

	if vc == nil {
		return "", engerr.InternalServerError("Failed to create container")
	}

	id, h, err := c.containerProxy.CreateContainerHandle(ctx, vc, config)
	if err != nil {
		return "", err
	}

	h, err = c.containerProxy.AddImageToContainer(ctx, h, id, vc.LayerID, vc.ImageID, config)
	if err != nil {
		return "", err
	}

	h, err = c.containerProxy.CreateContainerTask(ctx, h, id, vc.LayerID, config)
	if err != nil {
		return "", err
	}

	h, err = c.containerProxy.AddContainerToScope(ctx, h, config)
	if err != nil {
		return id, err
	}

	h, err = c.containerProxy.AddInteractionToContainer(ctx, h, config)
	if err != nil {
		return id, err
	}

	h, err = c.containerProxy.AddLoggingToContainer(ctx, h, config)
	if err != nil {
		return id, err
	}

	h, err = c.storageProxy.AddVolumesToContainer(ctx, h, config)
	if err != nil {
		return id, err
	}

	err = c.containerProxy.CommitContainerHandle(ctx, h, id, -1)
	if err != nil {
		return id, err
	}

	return id, nil
}

// ContainerKill sends signal to the container
// If no signal is given (sig 0), then Kill with SIGKILL and wait
// for the container to exit.
// If a signal is given, then just send it to the container and return.
func (c *ContainerBackend) ContainerKill(name string, sig uint64) error {
	defer trace.End(trace.Begin(fmt.Sprintf("%s, %d", name, sig)))

	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return engerr.NotFoundError(name)
	}

	err := c.containerProxy.Signal(ctx, vc, sig)
	if err == nil {
		actor := CreateContainerEventActorWithAttributes(vc, map[string]string{"signal": fmt.Sprintf("%d", sig)})

		EventService().Log(containerKillEvent, eventtypes.ContainerEventType, actor)

	}

	return err
}

// ContainerPause pauses a container
func (c *ContainerBackend) ContainerPause(name string) error {
	return engerr.APINotSupportedMsg(ProductName(), "ContainerPause")
}

// ContainerResize changes the size of the TTY of the process running
// in the container with the given name to the given height and width.
func (c *ContainerBackend) ContainerResize(name string, height, width int) error {
	defer trace.End(trace.Begin(name))

	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return engerr.NotFoundError(name)
	}

	// Call the port layer to resize
	plHeight := int32(height)
	plWidth := int32(width)

	var err error
	if err = c.containerProxy.Resize(ctx, vc.ContainerID, plHeight, plWidth); err == nil {
		actor := CreateContainerEventActorWithAttributes(vc, map[string]string{
			"height": fmt.Sprintf("%d", height),
			"width":  fmt.Sprintf("%d", width),
		})

		EventService().Log(containerResizeEvent, eventtypes.ContainerEventType, actor)
	}

	return err
}

// ContainerRestart stops and starts a container. It attempts to
// gracefully stop the container within the given timeout, forcefully
// stopping it if the timeout is exceeded. If given a negative
// timeout, ContainerRestart will wait forever until a graceful
// stop. Returns an error if the container cannot be found, or if
// there is an underlying error at any stage of the restart.
func (c *ContainerBackend) ContainerRestart(name string, seconds *int) error {
	op := trace.NewOperation(context.Background(), "ContainerRestart - %s", name)
	defer trace.End(trace.Begin(name, op))

	// Look up the container name in the metadata cache ot get long ID
	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return engerr.NotFoundError(name)
	}

	operation := func() error {
		return c.containerProxy.Stop(ctx, vc, name, seconds, false)
	}
	if err := retry.Do(operation, engerr.IsConflictError); err != nil {
		return engerr.InternalServerError(fmt.Sprintf("Stop failed with: %s", err))
	}

	operation = func() error {
		return c.containerStart(op, name, nil, true)
	}
	if err := retry.Do(operation, engerr.IsConflictError); err != nil {
		return engerr.InternalServerError(fmt.Sprintf("Start failed with: %s", err))
	}

	actor := CreateContainerEventActorWithAttributes(vc, map[string]string{})
	EventService().Log(containerRestartEvent, eventtypes.ContainerEventType, actor)

	return nil
}

// ContainerRm removes the container id from the filesystem. An error
// is returned if the container is not found, or if the remove
// fails. If the remove succeeds, the container name is released, and
// vicnetwork links are removed.
func (c *ContainerBackend) ContainerRm(name string, config *types.ContainerRmConfig) error {
	defer trace.End(trace.Begin(name))

	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return engerr.NotFoundError(name)
	}
	id := vc.ContainerID
	secs := 0
	running := false

	// Use the force and stop the container first
	if config.ForceRemove {
		if err := c.ContainerStop(name, &secs); err != nil {
			return err
		}
	} else {
		state, err := c.containerProxy.State(ctx, vc)
		if err != nil {
			if engerr.IsNotFoundError(err) {
				// remove container from persona cache, but don't return error to the user
				cache.ContainerCache().DeleteContainer(id)
				return nil
			}
			return engerr.InternalServerError(err.Error())
		}

		switch state.Status {
		case proxy.ContainerError:
			// force stop if container state is error to make sure container is deletable later
			c.containerProxy.Stop(ctx, vc, name, &secs, true)
		case "Starting":
			// if we are starting let the user know they must use the force
			return derr.NewRequestConflictError(fmt.Errorf("The container is starting.  To remove use -f"))
		case proxy.ContainerRunning:
			running = true
		}

		handle, err := c.containerProxy.Handle(ctx, id, name)
		if err != nil {
			return err
		}

		_, err = c.containerProxy.UnbindContainerFromNetwork(ctx, vc, handle)
		if err != nil {
			return err
		}
	}

	// Retry remove operation if container is not in running state.  If in running state, we only try
	// once to prevent retries from degrading performance.
	if !running {
		operation := func() error {
			return c.containerProxy.Remove(ctx, vc, config)
		}

		return retry.Do(operation, engerr.IsConflictError)
	}

	return c.containerProxy.Remove(ctx, vc, config)
}

// cleanupPortBindings gets port bindings for the container and
// unmaps ports if the cVM that previously bound them isn't powered on
func (c *ContainerBackend) cleanupPortBindings(vc *viccontainer.VicContainer) error {
	defer trace.End(trace.Begin(vc.ContainerID))
	for ctrPort, hostPorts := range vc.HostConfig.PortBindings {
		for _, hostPort := range hostPorts {
			hPort := hostPort.HostPort

			mappedCtr, mapped := network.ContainerWithPort(hPort)
			if !mapped {
				continue
			}

			log.Debugf("Container %q maps host port %s to container port %s", mappedCtr, hPort, ctrPort)
			// check state of the previously bound container with PL
			cc := cache.ContainerCache().GetContainer(mappedCtr)
			if cc == nil {
				// The container was removed from the cache and
				// port bindings were cleaned up by another operation.
				continue
			}
			state, err := c.containerProxy.State(ctx, cc)
			if err != nil {
				if engerr.IsNotFoundError(err) {
					log.Debugf("container(%s) not found in portLayer, removing from persona cache", cc.ContainerID)
					// we have a container in the persona cache, but it's been removed from the portLayer
					// which is the source of truth -- so remove from the persona cache after this func
					// completes
					defer cache.ContainerCache().DeleteContainer(cc.ContainerID)
				} else {
					// we have issues of an unknown variety...return..
					return engerr.InternalServerError(err.Error())
				}
			}

			if state != nil && state.Running {
				log.Debugf("Running container %q still holds port %s", mappedCtr, hPort)
				continue
			}

			log.Debugf("Unmapping ports for powered off / removed container %q", mappedCtr)
			err = network.UnmapPorts(cc.ContainerID, vc)
			if err != nil {
				return fmt.Errorf("Failed to unmap host port %s for container %q: %s",
					hPort, mappedCtr, err)
			}
		}
	}
	return nil
}

// ContainerStart starts a container.
func (c *ContainerBackend) ContainerStart(name string, hostConfig *containertypes.HostConfig, checkpoint string, checkpointDir string) error {
	op := trace.NewOperation(context.Background(), "ContainerStart - %s", name)
	defer trace.End(trace.Begin(name, op))

	operation := func() error {
		return c.containerStart(op, name, hostConfig, true)
	}
	if err := retry.Do(operation, engerr.IsConflictError); err != nil {
		op.Debugf("Container start failed due to error - %s", err.Error())
		return err
	}

	return nil
}

func (c *ContainerBackend) containerStart(op trace.Operation, name string, hostConfig *containertypes.HostConfig, bind bool) error {
	var err error

	// Get an API client to the portlayer
	client := PortLayerClient()

	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return engerr.NotFoundError(name)
	}
	if !vc.TryLock(APITimeout) {
		return engerr.ConcurrentAPIError(name, "ContainerStart")
	}
	defer vc.Unlock()
	id := vc.ContainerID
	op.Debugf("Obtained container lock for %s", id)

	// handle legacy hostConfig
	if hostConfig != nil {
		// hostConfig exist for backwards compatibility.  TODO: Figure out which parameters we
		// need to look at in hostConfig
	} else if vc != nil {
		hostConfig = vc.HostConfig
	}

	if vc != nil && hostConfig.NetworkMode.NetworkName() == "" {
		hostConfig.NetworkMode = vc.HostConfig.NetworkMode
	}

	// get a handle to the container
	handle, err := c.containerProxy.Handle(ctx, id, name)
	if err != nil {
		return err
	}

	var endpoints []*models.EndpointConfig
	// bind vicnetwork
	if bind {
		op.Debugf("Binding vicnetwork to container %s", id)

		var bindRes *scopes.BindContainerOK
		bindRes, err = client.Scopes.BindContainer(scopes.NewBindContainerParamsWithContext(ctx).WithHandle(handle))
		if err != nil {
			switch err := err.(type) {
			case *scopes.BindContainerNotFound:
				cache.ContainerCache().DeleteContainer(id)
				return engerr.NotFoundError(name)
			case *scopes.BindContainerInternalServerError:
				return engerr.InternalServerError(err.Payload.Message)
			default:
				return engerr.InternalServerError(err.Error())
			}
		}

		handle = bindRes.Payload.Handle
		endpoints = bindRes.Payload.Endpoints

		// unbind in case we fail later
		defer func() {
			if err != nil {
				op.Debugf("Unbinding %s due to error - %s", id, err.Error())
				client.Scopes.UnbindContainer(scopes.NewUnbindContainerParamsWithContext(ctx).WithHandle(handle))
			}
		}()

		// unmap ports that vc needs if they're not being used by previously mapped container
		err = c.cleanupPortBindings(vc)
		if err != nil {
			return err
		}
	}

	// change the state of the container
	// TODO: We need a resolved ID from the name
	op.Debugf("Setting container %s state to running", id)
	var stateChangeRes *containers.StateChangeOK
	stateChangeRes, err = client.Containers.StateChange(containers.NewStateChangeParamsWithContext(ctx).WithHandle(handle).WithState("RUNNING"))
	if err != nil {
		switch err := err.(type) {
		case *containers.StateChangeNotFound:
			cache.ContainerCache().DeleteContainer(id)
			return engerr.NotFoundError(name)
		case *containers.StateChangeDefault:
			return engerr.InternalServerError(err.Payload.Message)
		default:
			return engerr.InternalServerError(err.Error())
		}
	}

	handle = stateChangeRes.Payload

	// map ports
	if bind {
		scope, e := c.findPortBoundNetworkEndpoint(hostConfig, endpoints)
		if scope != nil && scope.ScopeType == constants.BridgeScopeType {
			if err = network.MapPorts(vc, e, id); err != nil {
				return engerr.InternalServerError(fmt.Sprintf("error mapping ports: %s", err))
			}

			defer func() {
				if err != nil {
					op.Debugf("Unbinding ports for %s due to error - %s", id, err.Error())
					network.UnmapPorts(id, vc)
				}
			}()
		}
	}

	// commit the handle; this will reconfigure and start the vm
	op.Debugf("Commit container %s", id)
	_, err = client.Containers.Commit(containers.NewCommitParamsWithContext(ctx).WithHandle(handle))
	if err != nil {
		switch err := err.(type) {
		case *containers.CommitNotFound:
			cache.ContainerCache().DeleteContainer(id)
			return engerr.NotFoundError(name)
		case *containers.CommitConflict:
			return engerr.ConflictError(err.Payload.Message)
		case *containers.CommitDefault:
			return engerr.InternalServerError(err.Payload.Message)
		default:
			return engerr.InternalServerError(err.Error())
		}
	}

	// Started event will be published on confirmation of successful start, triggered by port layer event stream

	return nil
}

func (c *ContainerBackend) defaultScope() string {
	defaultScope.Lock()
	defer defaultScope.Unlock()

	if defaultScope.scope != "" {
		return defaultScope.scope
	}

	client := PortLayerClient()
	listRes, err := client.Scopes.List(scopes.NewListParamsWithContext(ctx).WithIDName("default"))
	if err != nil {
		log.Error(err)
		return ""
	}

	if len(listRes.Payload) != 1 {
		log.Errorf("could not get default scope name")
		return ""
	}

	defaultScope.scope = listRes.Payload[0].Name
	return defaultScope.scope
}

func (c *ContainerBackend) findPortBoundNetworkEndpoint(hostconfig *containertypes.HostConfig, endpoints []*models.EndpointConfig) (*models.ScopeConfig, *models.EndpointConfig) {
	if len(hostconfig.PortBindings) == 0 {
		return nil, nil
	}

	// check if the port binding vicnetwork is a bridge type
	listRes, err := PortLayerClient().Scopes.List(scopes.NewListParamsWithContext(ctx).WithIDName(hostconfig.NetworkMode.NetworkName()))
	if err != nil {
		log.Error(err)
		return nil, nil
	}

	if l := len(listRes.Payload); l != 1 {
		log.Warnf("found %d scopes", l)
		return nil, nil
	}

	if listRes.Payload[0].ScopeType != constants.BridgeScopeType {
		log.Warnf("port binding for vicnetwork %s is not bridge type", hostconfig.NetworkMode.NetworkName())
		return listRes.Payload[0], nil
	}

	// look through endpoints to find the container's IP on the vicnetwork that has the port binding
	for _, e := range endpoints {
		if hostconfig.NetworkMode.NetworkName() == e.Scope || (hostconfig.NetworkMode.IsDefault() && e.Scope == c.defaultScope()) {
			return listRes.Payload[0], e
		}
	}

	return nil, nil
}

// ContainerStop looks for the given container and terminates it,
// waiting the given number of seconds before forcefully killing the
// container. If a negative number of seconds is given, ContainerStop
// will wait for a graceful termination. An error is returned if the
// container is not found, is already stopped, or if there is a
// problem stopping the container.
func (c *ContainerBackend) ContainerStop(name string, seconds *int) error {
	defer trace.End(trace.Begin(name))

	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return engerr.NotFoundError(name)
	}

	if seconds == nil {
		timeout := DefaultStopTimeout
		if vc.Config.StopTimeout != nil {
			timeout = *vc.Config.StopTimeout
		}
		seconds = &timeout
	}

	operation := func() error {
		return c.containerProxy.Stop(ctx, vc, name, seconds, true)
	}

	config := retry.NewBackoffConfig()
	config.MaxElapsedTime = maxElapsedTime
	if err := retry.DoWithConfig(operation, engerr.IsConflictError, config); err != nil {
		return err
	}

	actor := CreateContainerEventActorWithAttributes(vc, map[string]string{})
	EventService().Log(containerStopEvent, eventtypes.ContainerEventType, actor)

	return nil
}

// ContainerUnpause unpauses a container
func (c *ContainerBackend) ContainerUnpause(name string) error {
	return engerr.APINotSupportedMsg(ProductName(), "ContainerUnpause")
}

// ContainerUpdate updates configuration of the container
func (c *ContainerBackend) ContainerUpdate(name string, hostConfig *containertypes.HostConfig) (containertypes.ContainerUpdateOKBody, error) {
	return containertypes.ContainerUpdateOKBody{}, engerr.APINotSupportedMsg(ProductName(), "ContainerUpdate")
}

// ContainerWait stops processing until the given container is
// stopped. If the container is not found, an error is returned. On a
// successful stop, the exit code of the container is returned. On a
// timeout, an error is returned. If you want to wait forever, supply
// a negative duration for the timeout.
func (c *ContainerBackend) ContainerWait(name string, timeout time.Duration) (int, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("name(%s):timeout(%s)", name, timeout)))

	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return -1, engerr.NotFoundError(name)
	}

	dockerState, err := c.containerProxy.Wait(ctx, vc, timeout)
	if err != nil {
		return -1, err
	}

	return dockerState.ExitCode, nil
}

// docker's container.monitorBackend

// ContainerChanges returns a list of container fs changes
func (c *ContainerBackend) ContainerChanges(name string) ([]docker.Change, error) {
	defer trace.End(trace.Begin(name))
	op := trace.NewOperation(context.Background(), "ContainerChanges: %s", name)

	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return nil, engerr.NotFoundError(name)
	}

	r, err := c.GetContainerChanges(op, vc, false)
	if err != nil {
		return nil, engerr.InternalServerError(err.Error())
	}

	changes := []docker.Change{}

	tarFile := tar.NewReader(r)

	defer r.Close()

	for {
		hdr, err := tarFile.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return []docker.Change{}, engerr.InternalServerError(err.Error())
		}

		change := docker.Change{
			Path: filepath.Join("/", hdr.Name),
		}
		switch hdr.Xattrs[archive.ChangeTypeKey] {
		case "A":
			change.Kind = docker.ChangeAdd
		case "D":
			change.Kind = docker.ChangeDelete
			path := strings.TrimSuffix(change.Path, "/")
			p := strings.TrimPrefix(filepath.Base(path), docker.WhiteoutPrefix)
			change.Path = filepath.Join(filepath.Dir(path), p)
		case "C":
			change.Kind = docker.ChangeModify
		default:
			return []docker.Change{}, engerr.InternalServerError("Invalid change type")
		}
		changes = append(changes, change)
	}
	return changes, nil
}

// GetContainerChanges returns container changes from portlayer.
// Set data to true will return file data, otherwise, only return file headers with change type.
func (c *ContainerBackend) GetContainerChanges(op trace.Operation, vc *viccontainer.VicContainer, data bool) (io.ReadCloser, error) {
	host, err := sys.UUID()
	if err != nil {
		return nil, engerr.InternalServerError("Failed to determine host UUID")
	}

	parent := vc.LayerID
	spec := archive.FilterSpec{
		Inclusions: make(map[string]struct{}),
		Exclusions: make(map[string]struct{}),
	}

	r, err := archiveProxy.ArchiveExportReader(op, constants.ContainerStoreName, host, vc.ContainerID, parent, data, spec)
	if err != nil {
		return nil, engerr.InternalServerError(err.Error())
	}

	return r, nil
}

// ContainerInspect returns low-level information about a
// container. Returns an error if the container cannot be found, or if
// there is an error getting the data.
func (c *ContainerBackend) ContainerInspect(name string, size bool, version string) (interface{}, error) {
	// Ignore version.  We're supporting post-1.20 version.
	defer trace.End(trace.Begin(name))

	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return nil, engerr.NotFoundError(name)
	}
	id := vc.ContainerID
	log.Debugf("Found %q in cache as %q", id, vc.ContainerID)

	client := PortLayerClient()

	results, err := client.Containers.GetContainerInfo(containers.NewGetContainerInfoParamsWithContext(ctx).WithID(id))
	if err != nil {
		switch err := err.(type) {
		case *containers.GetContainerInfoNotFound:
			cache.ContainerCache().DeleteContainer(id)
			return nil, engerr.NotFoundError(name)
		case *containers.GetContainerInfoInternalServerError:
			return nil, engerr.InternalServerError(err.Payload.Message)
		default:
			return nil, engerr.InternalServerError(err.Error())
		}
	}

	inspectJSON, err := proxy.ContainerInfoToDockerContainerInspect(vc, results.Payload, PortLayerName())
	if err != nil {
		log.Errorf("containerInfoToDockerContainerInspect failed with %s", err)
		return nil, err
	}

	log.Debugf("ContainerInspect json config = %+v\n", inspectJSON.Config)

	return inspectJSON, nil
}

// ContainerLogs hooks up a container's stdout and stderr streams
// configured with the given struct.
func (c *ContainerBackend) ContainerLogs(ctx context.Context, name string, config *backend.ContainerLogsConfig, started chan struct{}) error {
	defer trace.End(trace.Begin(""))

	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return engerr.NotFoundError(name)
	}
	name = vc.ContainerID

	tailLines, since, err := c.validateContainerLogsConfig(vc, config)
	if err != nil {
		return err
	}

	// Outstream modification (from Docker's code) so the stream is streamed with the
	// necessary headers that the CLI expects.  This is Docker's scheme.
	wf := ioutils.NewWriteFlusher(config.OutStream)
	defer wf.Close()

	wf.Flush()

	outStream := io.Writer(wf)
	if !vc.Config.Tty {
		outStream = stdcopy.NewStdWriter(outStream, stdcopy.Stdout)
	}

	// Make a call to our proxy to handle the remoting
	err = c.streamProxy.StreamContainerLogs(ctx, name, outStream, started, config.Timestamps, config.Follow, since, tailLines)
	if err != nil {
		// Don't return an error encountered while streaming logs.
		// Once we've started streaming logs, the Docker client doesn't expect
		// an error to be returned as it leads to a malformed response body.
		log.Errorf("error while streaming logs: %#v", err)
	}

	return nil
}

// ContainerStats writes information about the container to the stream
// given in the config object.
func (c *ContainerBackend) ContainerStats(ctx context.Context, name string, config *backend.ContainerStatsConfig) error {
	defer trace.End(trace.Begin(name))

	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return engerr.NotFoundError(name)
	}

	// get the configured CPUMhz for this VCH so that we can calculate docker CPU stats
	cpuMhz, err := systemBackend.SystemCPUMhzLimit()
	if err != nil {
		// wrap error to provide a bit more detail
		sysErr := fmt.Errorf("unable to gather system CPUMhz for container(%s): %s", vc.ContainerID, err)
		log.Error(sysErr)
		return engerr.InternalServerError(sysErr.Error())
	}

	out := config.OutStream
	if config.Stream {
		// Outstream modification (from Docker's code) so the stream is streamed with the
		// necessary headers that the CLI expects.  This is Docker's scheme.
		wf := ioutils.NewWriteFlusher(config.OutStream)
		defer wf.Close()
		wf.Flush()
		out = io.Writer(wf)
	}

	// stats configuration
	statsConfig := &convert.ContainerStatsConfig{
		VchMhz:      cpuMhz,
		Stream:      config.Stream,
		ContainerID: vc.ContainerID,
		Out:         out,
		Memory:      vc.HostConfig.Memory,
	}

	// if we are not streaming then we need to get the container state
	if !config.Stream {
		statsConfig.ContainerState, err = c.containerProxy.State(ctx, vc)
		if err != nil {
			return engerr.InternalServerError(err.Error())
		}

	}

	err = c.streamProxy.StreamContainerStats(ctx, statsConfig)
	if err != nil {
		log.Errorf("error while streaming container (%s) stats: %s", vc.ContainerID, err)
	}
	return nil
}

// ContainerTop lists the processes running inside of the given
// container by calling ps with the given args, or with the flags
// "-ef" if no args are given.  An error is returned if the container
// is not found, or is not running, or if there are any problems
// running ps, or parsing the output.
func (c *ContainerBackend) ContainerTop(name string, psArgs string) (*types.ContainerProcessList, error) {
	return nil, engerr.APINotSupportedMsg(ProductName(), "ContainerTop")
}

// Containers returns the list of containers to show given the user's filtering.
func (c *ContainerBackend) Containers(config *types.ContainerListOptions) ([]*types.Container, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("ListOptions %#v", config)))

	// validate filters for support and validity
	listContext, err := filter.ValidateContainerFilters(config, acceptedPsFilterTags, unSupportedPsFilters)
	if err != nil {
		return nil, err
	}

	// Get an API client to the portlayer
	client := PortLayerClient()

	containme, err := client.Containers.GetContainerList(containers.NewGetContainerListParamsWithContext(ctx).WithAll(&listContext.All))
	if err != nil {
		switch err := err.(type) {

		case *containers.GetContainerListInternalServerError:
			return nil, fmt.Errorf("Error invoking GetContainerList: %s", err.Payload.Message)

		default:
			return nil, fmt.Errorf("Error invoking GetContainerList: %s", err.Error())
		}
	}
	// TODO: move to conversion function
	containers := make([]*types.Container, 0, len(containme.Payload))

payloadLoop:
	for _, t := range containme.Payload {

		// get this containers state
		dockerState := convert.State(t)

		var labels map[string]string
		if config.Filters.Include("label") {
			err = convert.ContainerAnnotation(t.ContainerConfig.Annotations, convert.AnnotationKeyLabels, &labels)
			if err != nil {
				return nil, fmt.Errorf("unable to convert vic annotations to docker labels (%s)", t.ContainerConfig.ContainerID)
			}
		}
		listContext.Labels = labels
		listContext.ExitCode = dockerState.ExitCode
		listContext.ID = t.ContainerConfig.ContainerID

		// prior to further conversion lets determine if this container
		// is needed or if the list is complete -- if the container is
		// needed conversion will continue and the container will be added to the
		// return array
		action := filter.IncludeContainer(listContext, t)
		switch action {
		case filter.ExcludeAction:
			// skip to next container
			continue payloadLoop
		case filter.StopAction:
			// we're done
			break payloadLoop
		}

		cmd := strings.Join(t.ProcessConfig.ExecArgs, " ")
		// the docker client expects the friendly name to be prefixed
		// with a forward slash -- create a new slice and add here
		names := make([]string, 0, len(t.ContainerConfig.Names))
		for i := range t.ContainerConfig.Names {
			names = append(names, clientFriendlyContainerName(t.ContainerConfig.Names[i]))
		}

		var ports []types.Port
		if dockerState.Running {
			// we only present port information in ps output when the container is running and
			// should be responsive at that address:port
			ports = network.DirectPortInformation(t)

			ips, err := network.PublicIPv4Addrs()
			if err != nil {
				log.Errorf("Could not get IP information for reporting port bindings: %s", err)
				// display port mappings without IP data if we cannot get it
				ips = []string{""}
			}
			c := cache.ContainerCache().GetContainer(t.ContainerConfig.ContainerID)
			pi := network.PortForwardingInformation(c, ips)
			if pi != nil {
				ports = append(ports, pi...)
			}
		}

		// verify that the repo:tag exists for the container -- if it doesn't then we should present the
		// truncated imageID -- if we have a failure determining then we'll show the data we have
		repo := *t.ContainerConfig.RepoName
		// #nosec: Errors unhandled.
		ref, _ := reference.ParseNamed(*t.ContainerConfig.RepoName)
		if ref != nil {
			imageID, err := cache.RepositoryCache().Get(ref)
			if err != nil && err == cache.ErrDoesNotExist {
				// the tag has been removed, so we need to show the truncated imageID
				imageID = cache.RepositoryCache().GetImageID(t.ContainerConfig.LayerID)
				if imageID != "" {
					id := uid.Parse(imageID)
					repo = id.Truncate().String()
				}
			}
		}

		c := &types.Container{
			ID:      t.ContainerConfig.ContainerID,
			Image:   repo,
			Created: time.Unix(0, t.ContainerConfig.CreateTime).Unix(),
			Status:  dockerState.Status,
			Names:   names,
			Command: cmd,
			SizeRw:  t.ContainerConfig.StorageSize,
			Ports:   ports,
			State:   filter.DockerState(t.ContainerConfig.State),
		}

		// The container should be included in the list
		containers = append(containers, c)
		listContext.Counter++

	}

	return containers, nil
}

func (c *ContainerBackend) ContainersPrune(pruneFilters filters.Args) (*types.ContainersPruneReport, error) {
	return nil, engerr.APINotSupportedMsg(ProductName(), "ContainersPrune")
}

// docker's container.attachBackend

// ContainerAttach attaches to logs according to the config passed in. See ContainerAttachConfig.
func (c *ContainerBackend) ContainerAttach(name string, ca *backend.ContainerAttachConfig) error {
	defer trace.End(trace.Begin(name))

	operation := func() error {
		return c.containerAttach(name, ca)
	}
	if err := retry.Do(operation, engerr.IsConflictError); err != nil {
		return err
	}
	return nil
}

func (c *ContainerBackend) containerAttach(name string, ca *backend.ContainerAttachConfig) error {
	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return engerr.NotFoundError(name)

	}
	id := vc.ContainerID

	handle, err := c.containerProxy.Handle(ctx, id, name)
	if err != nil {
		return err
	}

	handleprime, err := c.containerProxy.BindInteraction(ctx, handle, name, id)
	if err != nil {
		return err
	}

	if err := c.containerProxy.CommitContainerHandle(ctx, handleprime, name, 0); err != nil {
		return err
	}

	stdin, stdout, stderr, err := ca.GetStreams()
	if err != nil {
		return engerr.InternalServerError("Unable to get stdio streams for calling client")
	}
	defer stdin.Close()

	if !vc.Config.Tty && ca.MuxStreams {
		// replace the stdout/stderr with Docker's multiplex stream
		if ca.UseStderr {
			stderr = stdcopy.NewStdWriter(stderr, stdcopy.Stderr)
		}
		if ca.UseStdout {
			stdout = stdcopy.NewStdWriter(stdout, stdcopy.Stdout)
		}
	}

	actor := CreateContainerEventActorWithAttributes(vc, map[string]string{})
	EventService().Log(containerAttachEvent, eventtypes.ContainerEventType, actor)

	if vc.Config.Tty {
		ca.UseStderr = false
	}

	ac := &proxy.AttachConfig{
		ID: id,
		ContainerAttachConfig: ca,
		UseTty:                vc.Config.Tty,
		CloseStdin:            vc.Config.StdinOnce,
	}

	err = c.streamProxy.AttachStreams(context.Background(), ac, stdin, stdout, stderr)
	if err != nil {
		if _, ok := err.(engerr.DetachError); ok {
			log.Infof("Detach detected, tearing down connection")

			// fire detach event
			EventService().Log(containerDetachEvent, eventtypes.ContainerEventType, actor)

			// DON'T UNBIND FOR NOW, UNTIL/UNLESS REFERENCE COUNTING IS IN PLACE
			// This avoids cutting the communication channel for other sessions connected to this
			// container

			// FIXME: call UnbindInteraction/Commit
		}
		return err
	}

	return nil
}

// ContainerRename changes the name of a container, using the oldName
// to find the container. An error is returned if newName is already
// reserved.
func (c *ContainerBackend) ContainerRename(oldName, newName string) error {
	defer trace.End(trace.Begin(newName))

	if oldName == "" || newName == "" {
		err := fmt.Errorf("neither old nor new names may be empty")
		log.Errorf("%s", err.Error())
		return derr.NewErrorWithStatusCode(err, http.StatusInternalServerError)
	}

	if !utils.RestrictedNamePattern.MatchString(newName) {
		err := fmt.Errorf("invalid container name (%s), only %s are allowed", newName, utils.RestrictedNameChars)
		log.Errorf("%s", err.Error())
		return derr.NewErrorWithStatusCode(err, http.StatusInternalServerError)
	}

	// Look up the container name in the metadata cache to get long ID
	vc := cache.ContainerCache().GetContainer(oldName)
	if vc == nil {
		log.Errorf("Container %s not found", oldName)
		return engerr.NotFoundError(oldName)
	}

	oldName = vc.Name
	if oldName == newName {
		err := fmt.Errorf("renaming a container with the same name as its current name")
		log.Errorf("%s", err.Error())
		return derr.NewErrorWithStatusCode(err, http.StatusInternalServerError)
	}

	// reserve the new name in containerCache
	if err := cache.ContainerCache().ReserveName(vc, newName); err != nil {
		log.Errorf("%s", err.Error())
		return derr.NewRequestConflictError(err)
	}

	renameOp := func() error {
		return c.containerProxy.Rename(ctx, vc, newName)
	}

	if err := retry.Do(renameOp, engerr.IsConflictError); err != nil {
		log.Errorf("Rename error: %s", err)
		cache.ContainerCache().ReleaseName(newName)
		return err
	}

	// update containerCache
	if err := cache.ContainerCache().UpdateContainerName(oldName, newName); err != nil {
		log.Errorf("Failed to update container cache: %s", err)
		cache.ContainerCache().ReleaseName(newName)
		return err
	}

	log.Infof("Container %s renamed to %s", oldName, newName)

	actor := CreateContainerEventActorWithAttributes(vc, map[string]string{"newName": fmt.Sprintf("%s", newName)})

	EventService().Log("Rename", eventtypes.ContainerEventType, actor)

	return nil
}

// helper function to format the container name
// to the docker client approved format
func clientFriendlyContainerName(name string) string {
	return fmt.Sprintf("/%s", name)
}

//------------------------------------
// ContainerCreate() Utility Functions
//------------------------------------

// createInternalVicContainer() creates an container representation (for docker personality)
// This is called by ContainerCreate()
func createInternalVicContainer(image *metadata.ImageConfig, config *types.ContainerCreateConfig) (*viccontainer.VicContainer, error) {
	// provide basic container config via the image
	container := viccontainer.NewVicContainer()
	container.LayerID = image.V1Image.ID // store childmost layer ID to map to the proper vmdk
	container.ImageID = image.ImageID
	container.Config = image.Config //Set defaults.  Overrides will get copied below.

	return container, nil
}

// SetConfigOptions is a place to add necessary container configuration
// values that were not explicitly supplied by the user
func setCreateConfigOptions(config, imageConfig *containertypes.Config) {
	// Overwrite or append the image's config from the CLI with the metadata from the image's
	// layer metadata where appropriate
	if len(config.Cmd) == 0 {
		config.Cmd = imageConfig.Cmd
	}
	if config.WorkingDir == "" {
		config.WorkingDir = imageConfig.WorkingDir
	}
	if len(config.Entrypoint) == 0 {
		config.Entrypoint = imageConfig.Entrypoint
	}

	if config.Volumes == nil {
		config.Volumes = imageConfig.Volumes
	} else {
		for k, v := range imageConfig.Volumes {
			//NOTE: the value of the map is an empty struct.
			//      we also do not care about duplicates.
			//      This Volumes map is really a Set.
			config.Volumes[k] = v
		}
	}

	if config.User == "" {
		config.User = imageConfig.User
	}
	// set up environment
	config.Env = setEnvFromImageConfig(config.Tty, config.Env, imageConfig.Env)
}

func setEnvFromImageConfig(tty bool, env []string, imgEnv []string) []string {
	// Set PATH in ENV if needed
	env = setPathFromImageConfig(env, imgEnv)

	containerEnv := make(map[string]string, len(env))
	for _, e := range env {
		kv := strings.SplitN(e, "=", 2)
		var val string
		if len(kv) == 2 {
			val = kv[1]
		}
		containerEnv[kv[0]] = val
	}

	// Set TERM to xterm if tty is set, unless user supplied a different TERM
	if tty {
		if _, ok := containerEnv["TERM"]; !ok {
			env = append(env, "TERM=xterm")
		}
	}

	// add remaining environment variables from the image config to the container
	// config, taking care not to overwrite anything
	for _, imageEnv := range imgEnv {
		key := strings.SplitN(imageEnv, "=", 2)[0]
		// is environment variable already set in container config?
		if _, ok := containerEnv[key]; !ok {
			// no? let's copy it from the image config
			env = append(env, imageEnv)
		}
	}

	return env
}

func setPathFromImageConfig(env []string, imgEnv []string) []string {
	// check if user supplied PATH environment variable at creation time
	for _, v := range env {
		if strings.HasPrefix(v, "PATH=") {
			// a PATH is set, bail
			return env
		}
	}

	// check to see if the image this container is created from supplies a PATH
	for _, v := range imgEnv {
		if strings.HasPrefix(v, "PATH=") {
			// a PATH was found, add it to the config
			env = append(env, v)
			return env
		}
	}

	// no PATH set, use the default
	env = append(env, fmt.Sprintf("PATH=%s", defaultEnvPath))

	return env
}

// validateCreateConfig() checks the parameters for ContainerCreate().
// It may "fix up" the config param passed into ConntainerCreate() if needed.
func validateCreateConfig(config *types.ContainerCreateConfig) error {
	defer trace.End(trace.Begin("Container.validateCreateConfig"))

	if config.Config == nil {
		return engerr.BadRequestError("invalid config")
	}

	if config.HostConfig == nil {
		config.HostConfig = &containertypes.HostConfig{}
	}

	// process cpucount here
	var cpuCount int64 = DefaultCPUs

	// support windows client
	if config.HostConfig.CPUCount > 0 {
		cpuCount = config.HostConfig.CPUCount
	} else {
		// we hijack --cpuset-cpus in the non-windows case
		if config.HostConfig.CpusetCpus != "" {
			cpus := strings.Split(config.HostConfig.CpusetCpus, ",")
			if c, err := strconv.Atoi(cpus[0]); err == nil {
				cpuCount = int64(c)
			} else {
				return fmt.Errorf("Error parsing CPU count: %s", err)
			}
		}
	}
	config.HostConfig.CPUCount = cpuCount

	// fix-up cpu/memory settings here
	if cpuCount < MinCPUs {
		config.HostConfig.CPUCount = MinCPUs
	}
	log.Infof("Container CPU count: %d", config.HostConfig.CPUCount)

	// convert from bytes to MiB for vsphere
	memoryMB := config.HostConfig.Memory / units.MiB
	if memoryMB == 0 {
		memoryMB = MemoryDefaultMB
	} else if memoryMB < MemoryMinMB {
		memoryMB = MemoryMinMB
	}

	// check that memory is aligned
	if remainder := memoryMB % MemoryAlignMB; remainder != 0 {
		log.Warnf("Default container VM memory must be %d aligned for hotadd, rounding up.", MemoryAlignMB)
		memoryMB += MemoryAlignMB - remainder
	}

	config.HostConfig.Memory = memoryMB
	log.Infof("Container memory: %d MB", config.HostConfig.Memory)

	if config.NetworkingConfig == nil {
		config.NetworkingConfig = &dnetwork.NetworkingConfig{}
	} else {
		if l := len(config.NetworkingConfig.EndpointsConfig); l > 1 {
			return fmt.Errorf("NetworkMode error: Container can be connected to one vicnetwork endpoint only")
		}
		// If NetworkConfig exists, set NetworkMode to the default endpoint vicnetwork, assuming only one endpoint vicnetwork as the default vicnetwork during container create
		for networkName := range config.NetworkingConfig.EndpointsConfig {
			config.HostConfig.NetworkMode = containertypes.NetworkMode(networkName)
		}
	}

	// validate port bindings
	var ips []string
	if addrs, err := network.PublicIPv4Addrs(); err != nil {
		log.Warnf("could not get address for public interface: %s", err)
	} else {
		ips = make([]string, len(addrs))
		for i := range addrs {
			ips[i] = addrs[i]
		}
	}

	for _, pbs := range config.HostConfig.PortBindings {
		for _, pb := range pbs {
			if pb.HostIP != "" && pb.HostIP != "0.0.0.0" {
				// check if specified host ip equals any of the addresses on the "client" interface
				found := false
				for _, i := range ips {
					if i == pb.HostIP {
						found = true
						break
					}
				}
				if !found {
					return engerr.InternalServerError("host IP for port bindings is only supported for 0.0.0.0 and the public interface IP address")
				}
			}

			// #nosec: Errors unhandled.
			start, end, _ := nat.ParsePortRangeToInt(pb.HostPort)
			if start != end {
				return engerr.InternalServerError("host port ranges are not supported for port bindings")
			}
		}
	}

	// https://github.com/vmware/vic/issues/1378
	if len(config.Config.Entrypoint) == 0 && len(config.Config.Cmd) == 0 {
		return derr.NewRequestNotFoundError(fmt.Errorf("No command specified"))
	}

	return nil
}

func copyConfigOverrides(vc *viccontainer.VicContainer, config types.ContainerCreateConfig) {
	// Copy the create overrides to our new container
	vc.Name = config.Name
	vc.Config.Cmd = config.Config.Cmd
	vc.Config.WorkingDir = config.Config.WorkingDir
	vc.Config.Entrypoint = config.Config.Entrypoint
	vc.Config.Env = config.Config.Env
	vc.Config.AttachStdin = config.Config.AttachStdin
	vc.Config.AttachStdout = config.Config.AttachStdout
	vc.Config.AttachStderr = config.Config.AttachStderr
	vc.Config.Tty = config.Config.Tty
	vc.Config.OpenStdin = config.Config.OpenStdin
	vc.Config.StdinOnce = config.Config.StdinOnce
	vc.Config.StopSignal = config.Config.StopSignal
	vc.Config.Volumes = config.Config.Volumes
	vc.HostConfig = config.HostConfig
}

//----------------------------------
// ContainerLogs() utility functions
//----------------------------------

// validateContainerLogsConfig() validates and extracts options for logging from the
// backend.ContainerLogsConfig object we're given.
//
// returns:
//	tail lines, since (in unix time), error
func (c *ContainerBackend) validateContainerLogsConfig(vc *viccontainer.VicContainer, config *backend.ContainerLogsConfig) (int64, int64, error) {
	if !(config.ShowStdout || config.ShowStderr) {
		return 0, 0, fmt.Errorf("You must choose at least one stream")
	}

	unsupported := func(opt string) (int64, int64, error) {
		return 0, 0, fmt.Errorf("container %s does not support '--%s'", vc.ContainerID, opt)
	}

	tailLines := int64(-1)
	if config.Tail != "" && config.Tail != "all" {
		n, err := strconv.ParseInt(config.Tail, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("error parsing tail option: %s", err)
		}
		tailLines = n
	}

	var since time.Time
	if config.Since != "" {
		s, n, err := timetypes.ParseTimestamps(config.Since, 0)
		if err != nil {
			return 0, 0, err
		}
		since = time.Unix(s, n)
	}

	// TODO(jzt): this should not require an extra call to the portlayer. We should
	// update container.DataVersion when we hydrate the container cache at VCH startup
	// see https://github.com/vmware/vic/issues/4194
	if config.Timestamps {
		// check container DataVersion to make sure it's supported
		params := containers.NewGetContainerInfoParams()
		params.SetID(vc.ContainerID)
		info, err := PortLayerClient().Containers.GetContainerInfo(params)
		if err != nil {
			return 0, 0, err
		}
		if info.Payload.DataVersion == 0 {
			return unsupported("timestamps")
		}
	}

	return tailLines, since.Unix(), nil
}

func CreateContainerEventActorWithAttributes(vc *viccontainer.VicContainer, attributes map[string]string) eventtypes.Actor {
	if vc.Config != nil {
		for k, v := range vc.Config.Labels {
			attributes[k] = v
		}
	}
	if vc.Config.Image != "" {
		attributes["image"] = vc.Config.Image
	}
	attributes["name"] = strings.TrimLeft(vc.Name, "/")

	return eventtypes.Actor{
		ID:         vc.ContainerID,
		Attributes: attributes,
	}
}
