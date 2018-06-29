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

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/google/uuid"

	derr "github.com/docker/docker/api/errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/go-units"

	viccontainer "github.com/vmware/vic/lib/apiservers/engine/backends/container"
	"github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/containers"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/storage"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/pkg/trace"
)

type VicStorageProxy interface {
	Create(ctx context.Context, name, driverName string, volumeData, labels map[string]string) (*types.Volume, error)
	VolumeList(ctx context.Context, filter string) ([]*models.VolumeResponse, error)
	VolumeInfo(ctx context.Context, name string) (*models.VolumeResponse, error)
	Remove(ctx context.Context, name string) error

	AddVolumesToContainer(ctx context.Context, handle string, config types.ContainerCreateConfig) (string, error)
}

type StorageProxy struct {
	client *client.PortLayer
}

type volumeFields struct {
	ID    string
	Dest  string
	Flags string
}

type VolumeMetadata struct {
	Driver        string
	DriverOpts    map[string]string
	Name          string
	Labels        map[string]string
	AttachHistory []string
	Image         string
}

const (
	DriverArgFlagKey      = "flags"
	DriverArgContainerKey = "container"
	DriverArgImageKey     = "image"

	OptsVolumeStoreKey     string = "volumestore"
	OptsCapacityKey        string = "capacity"
	DockerMetadataModelKey string = "DockerMetaData"
)

// define a set (whitelist) of valid driver opts keys for command line argument validation
var validDriverOptsKeys = map[string]struct{}{
	OptsVolumeStoreKey:    {},
	OptsCapacityKey:       {},
	DriverArgFlagKey:      {},
	DriverArgContainerKey: {},
	DriverArgImageKey:     {},
}

// Volume drivers currently supported. "local" is the default driver supplied by the client
// and is equivalent to "vsphere" for our implementation.
var SupportedVolDrivers = map[string]struct{}{
	"vsphere": {},
	"local":   {},
}

//Validation pattern for Volume Names
var volumeNameRegex = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9_.-]*$")

func NewStorageProxy(client *client.PortLayer) VicStorageProxy {
	if client == nil {
		return nil
	}

	return &StorageProxy{client: client}
}

func (s *StorageProxy) Create(ctx context.Context, name, driverName string, volumeData, labels map[string]string) (*types.Volume, error) {
	defer trace.End(trace.Begin(""))

	if s.client == nil {
		return nil, errors.NillPortlayerClientError("StorageProxy")
	}

	result, err := s.volumeCreate(ctx, name, driverName, volumeData, labels)
	if err != nil {
		switch err := err.(type) {
		case *storage.CreateVolumeConflict:
			return result, errors.VolumeInternalServerError(fmt.Errorf("A volume named %s already exists. Choose a different volume name.", name))
		case *storage.CreateVolumeNotFound:
			return result, errors.VolumeInternalServerError(fmt.Errorf("No volume store named (%s) exists", volumeStore(volumeData)))
		case *storage.CreateVolumeInternalServerError:
			// FIXME: right now this does not return an error model...
			return result, errors.VolumeInternalServerError(fmt.Errorf("%s", err.Error()))
		case *storage.CreateVolumeDefault:
			return result, errors.VolumeInternalServerError(fmt.Errorf("%s", err.Payload.Message))
		default:
			return result, errors.VolumeInternalServerError(fmt.Errorf("%s", err))
		}
	}

	return result, nil
}

// volumeCreate issues a CreateVolume request to the portlayer
func (s *StorageProxy) volumeCreate(ctx context.Context, name, driverName string, volumeData, labels map[string]string) (*types.Volume, error) {
	defer trace.End(trace.Begin(""))
	result := &types.Volume{}

	if s.client == nil {
		return nil, errors.NillPortlayerClientError("StorageProxy")
	}

	if name == "" {
		name = uuid.New().String()
	}

	// TODO: support having another driver besides vsphere.
	// assign the values of the model to be passed to the portlayer handler
	req, varErr := newVolumeCreateReq(name, driverName, volumeData, labels)
	if varErr != nil {
		return result, varErr
	}
	log.Infof("Finalized model for volume create request to portlayer: %#v", req)

	res, err := s.client.Storage.CreateVolume(storage.NewCreateVolumeParamsWithContext(ctx).WithVolumeRequest(req))
	if err != nil {
		return result, err
	}

	return NewVolumeModel(res.Payload, labels), nil
}

func (s *StorageProxy) VolumeList(ctx context.Context, filter string) ([]*models.VolumeResponse, error) {
	defer trace.End(trace.Begin(""))

	if s.client == nil {
		return nil, errors.NillPortlayerClientError("StorageProxy")
	}

	res, err := s.client.Storage.ListVolumes(storage.NewListVolumesParamsWithContext(ctx).WithFilterString(&filter))
	if err != nil {
		switch err := err.(type) {
		case *storage.ListVolumesInternalServerError:
			return nil, errors.VolumeInternalServerError(fmt.Errorf("error from portlayer server: %s", err.Payload.Message))
		case *storage.ListVolumesDefault:
			return nil, errors.VolumeInternalServerError(fmt.Errorf("error from portlayer server: %s", err.Payload.Message))
		default:
			return nil, errors.VolumeInternalServerError(fmt.Errorf("error from portlayer server: %s", err.Error()))
		}
	}

	return res.Payload, nil
}

func (s *StorageProxy) VolumeInfo(ctx context.Context, name string) (*models.VolumeResponse, error) {
	defer trace.End(trace.Begin(name))

	if name == "" {
		return nil, nil
	}

	if s.client == nil {
		return nil, errors.NillPortlayerClientError("StorageProxy")
	}

	param := storage.NewGetVolumeParamsWithContext(ctx).WithName(name)
	res, err := s.client.Storage.GetVolume(param)
	if err != nil {
		switch err := err.(type) {
		case *storage.GetVolumeNotFound:
			return nil, errors.VolumeNotFoundError(name)
		default:
			return nil, errors.VolumeInternalServerError(fmt.Errorf("error from portlayer server: %s", err.Error()))
		}
	}

	return res.Payload, nil
}

func (s *StorageProxy) Remove(ctx context.Context, name string) error {
	defer trace.End(trace.Begin(name))

	if s.client == nil {
		return errors.NillPortlayerClientError("StorageProxy")
	}

	_, err := s.client.Storage.RemoveVolume(storage.NewRemoveVolumeParamsWithContext(ctx).WithName(name))
	if err != nil {
		switch err := err.(type) {
		case *storage.RemoveVolumeNotFound:
			return derr.NewRequestNotFoundError(fmt.Errorf("Get %s: no such volume", name))
		case *storage.RemoveVolumeConflict:
			return derr.NewRequestConflictError(fmt.Errorf(err.Payload.Message))
		case *storage.RemoveVolumeInternalServerError:
			return errors.VolumeInternalServerError(fmt.Errorf("Server error from portlayer: %s", err.Payload.Message))
		default:
			return errors.VolumeInternalServerError(fmt.Errorf("Server error from portlayer: %s", err))
		}
	}

	return nil
}

// AddVolumesToContainer adds volumes to a container, referenced by handle.
// If an error is returned, the returned handle should not be used.
//
// returns:
//	modified handle
func (s *StorageProxy) AddVolumesToContainer(ctx context.Context, handle string, config types.ContainerCreateConfig) (string, error) {
	defer trace.End(trace.Begin(handle))

	if s.client == nil {
		return "", errors.NillPortlayerClientError("StorageProxy")
	}

	// Volume Attachment Section
	log.Debugf("ContainerProxy.AddVolumesToContainer - VolumeSection")
	log.Debugf("Raw volume arguments: binds:  %#v, volumes: %#v", config.HostConfig.Binds, config.Config.Volumes)

	// Collect all volume mappings. In a docker create/run, they
	// can be anonymous (-v /dir) or specific (-v vol-name:/dir).
	// anonymous volumes can also come from Image Metadata

	rawAnonVolumes := make([]string, 0, len(config.Config.Volumes))
	for k := range config.Config.Volumes {
		rawAnonVolumes = append(rawAnonVolumes, k)
	}

	volList, err := finalizeVolumeList(config.HostConfig.Binds, rawAnonVolumes)
	if err != nil {
		return handle, errors.BadRequestError(err.Error())
	}
	log.Infof("Finalized volume list: %#v", volList)

	if len(config.Config.Volumes) > 0 {
		// override anonymous volume list with generated volume id
		for _, vol := range volList {
			if _, ok := config.Config.Volumes[vol.Dest]; ok {
				delete(config.Config.Volumes, vol.Dest)
				mount := getMountString(vol.ID, vol.Dest, vol.Flags)
				config.Config.Volumes[mount] = struct{}{}
				log.Debugf("Replace anonymous volume config %s with %s", vol.Dest, mount)
			}
		}
	}

	// Create and join volumes.
	for _, fields := range volList {
		// We only set these here for volumes made on a docker create
		volumeData := make(map[string]string)
		volumeData[DriverArgFlagKey] = fields.Flags
		volumeData[DriverArgContainerKey] = config.Name
		volumeData[DriverArgImageKey] = config.Config.Image

		// NOTE: calling volumeCreate regardless of whether the volume is already
		// present can be avoided by adding an extra optional param to VolumeJoin,
		// which would then call volumeCreate if the volume does not exist.
		_, err := s.volumeCreate(ctx, fields.ID, "vsphere", volumeData, nil)
		if err != nil {
			switch err := err.(type) {
			case *storage.CreateVolumeConflict:
				// Implicitly ignore the error where a volume with the same name
				// already exists. We can just join the said volume to the container.
				log.Infof("a volume with the name %s already exists", fields.ID)
			case *storage.CreateVolumeNotFound:
				return handle, errors.VolumeCreateNotFoundError(volumeStore(volumeData))
			default:
				return handle, errors.InternalServerError(err.Error())
			}
		} else {
			log.Infof("volumeCreate succeeded. Volume mount section ID: %s", fields.ID)
		}

		flags := make(map[string]string)
		//NOTE: for now we are passing the flags directly through. This is NOT SAFE and only a stop gap.
		flags[constants.Mode] = fields.Flags
		joinParams := storage.NewVolumeJoinParamsWithContext(ctx).WithJoinArgs(&models.VolumeJoinConfig{
			Flags:     flags,
			Handle:    handle,
			MountPath: fields.Dest,
		}).WithName(fields.ID)

		res, err := s.client.Storage.VolumeJoin(joinParams)
		if err != nil {
			switch err := err.(type) {
			case *storage.VolumeJoinInternalServerError:
				return handle, errors.InternalServerError(err.Payload.Message)
			case *storage.VolumeJoinDefault:
				return handle, errors.InternalServerError(err.Payload.Message)
			case *storage.VolumeJoinNotFound:
				return handle, errors.VolumeJoinNotFoundError(err.Payload.Message)
			default:
				return handle, errors.InternalServerError(err.Error())
			}
		}

		handle = res.Payload
	}

	return handle, nil
}

// allContainers obtains all containers from the portlayer, akin to `docker ps -a`.
func (s *StorageProxy) allContainers(ctx context.Context) ([]*models.ContainerInfo, error) {
	if s.client == nil {
		return nil, errors.NillPortlayerClientError("StorageProxy")
	}

	all := true
	cons, err := s.client.Containers.GetContainerList(containers.NewGetContainerListParamsWithContext(ctx).WithAll(&all))
	if err != nil {
		return nil, err
	}

	return cons.Payload, nil
}

// fetchJoinedVolumes obtains all containers from the portlayer and returns a map with all
// volumes that are joined to at least one container.
func (s *StorageProxy) fetchJoinedVolumes(ctx context.Context) (map[string]struct{}, error) {
	conts, err := s.allContainers(ctx)
	if err != nil {
		return nil, errors.VolumeInternalServerError(err)
	}

	joinedVolumes := make(map[string]struct{})
	var v struct{}
	for i := range conts {
		for _, vol := range conts[i].VolumeConfig {
			joinedVolumes[vol.Name] = v
		}
	}

	return joinedVolumes, nil
}

//------------------------------------
// Utility Functions
//------------------------------------

func NewVolumeModel(volume *models.VolumeResponse, labels map[string]string) *types.Volume {
	return &types.Volume{
		Driver:     volume.Driver,
		Name:       volume.Name,
		Labels:     labels,
		Mountpoint: volume.Label,
	}
}

// volumeStore returns the value of the optional volume store param specified in the CLI.
func volumeStore(args map[string]string) string {
	storeName, ok := args[OptsVolumeStoreKey]
	if !ok {
		return "default"
	}
	return storeName
}

// getMountString returns a colon-delimited string containing a volume's name/ID, mount
// point and flags.
func getMountString(mounts ...string) string {
	return strings.Join(mounts, ":")
}

func createVolumeMetadata(req *models.VolumeRequest, driverargs, labels map[string]string) (string, error) {
	metadata := VolumeMetadata{
		Driver:        req.Driver,
		DriverOpts:    req.DriverArgs,
		Name:          req.Name,
		Labels:        labels,
		AttachHistory: []string{driverargs[DriverArgContainerKey]},
		Image:         driverargs[DriverArgImageKey],
	}
	result, err := json.Marshal(metadata)
	return string(result), err
}

// RemoveAnonContainerVols removes anonymous volumes joined to a container. It is invoked
// once the said container has been removed. It fetches a list of volumes that are joined
// to at least one other container, and calls the portlayer to remove this container's
// anonymous volumes if they are dangling. Errors, if any, are only logged.
func RemoveAnonContainerVols(ctx context.Context, pl *client.PortLayer, cID string, vc *viccontainer.VicContainer) {
	// NOTE: these strings come in the form of <volume id>:<destination>:<volume options>
	volumes := vc.Config.Volumes
	// NOTE: these strings come in the form of <volume id>:<destination path>
	namedVolumes := vc.HostConfig.Binds

	// assemble a mask of volume paths before processing binds. MUST be paths, as we want to move to honoring the proper metadata in the "volumes" section in the future.
	namedMaskList := make(map[string]struct{}, 0)
	for _, entry := range namedVolumes {
		fields := strings.SplitN(entry, ":", 2)
		if len(fields) != 2 {
			log.Errorf("Invalid entry in the HostConfig.Binds metadata section for container %s: %s", cID, entry)
			continue
		}
		destPath := fields[1]
		namedMaskList[destPath] = struct{}{}
	}

	proxy := StorageProxy{client: pl}
	joinedVols, err := proxy.fetchJoinedVolumes(ctx)
	if err != nil {
		log.Errorf("Unable to obtain joined volumes from portlayer, skipping removal of anonymous volumes for %s: %s", cID, err.Error())
		return
	}

	for vol := range volumes {
		// Extract the volume ID from the full mount path, which is of form "id:mountpath:flags" - see getMountString().
		volFields := strings.SplitN(vol, ":", 3)

		// NOTE(mavery): this check will start to fail when we fix our metadata correctness issues
		if len(volFields) != 3 {
			log.Debugf("Invalid entry in the volumes metadata section for container %s: %s", cID, vol)
			continue
		}
		volName := volFields[0]
		volPath := volFields[1]

		_, isNamed := namedMaskList[volPath]
		_, joined := joinedVols[volName]
		if !joined && !isNamed {
			_, err := pl.Storage.RemoveVolume(storage.NewRemoveVolumeParamsWithContext(ctx).WithName(volName))
			if err != nil {
				log.Debugf("Unable to remove anonymous volume %s in container %s: %s", volName, cID, err.Error())
				continue
			}
			log.Debugf("Successfully removed anonymous volume %s during remove operation against container(%s)", volName, cID)
		}
	}
}

// processVolumeParam is used to turn any call from docker create -v <stuff> into a volumeFields object.
// The -v has 3 forms. -v <anonymous mount path>, -v <Volume Name>:<Destination Mount Path> and
// -v <Volume Name>:<Destination Mount Path>:<mount flags>
func processVolumeParam(volString string) (volumeFields, error) {
	volumeStrings := strings.Split(volString, ":")
	fields := volumeFields{}

	// Error out if the intended volume is a directory on the client filesystem.
	numVolParams := len(volumeStrings)
	if numVolParams > 1 && strings.HasPrefix(volumeStrings[0], "/") {
		return volumeFields{}, errors.InvalidVolumeError{}
	}

	// This switch determines which type of -v was invoked.
	switch numVolParams {
	case 1:
		VolumeID, err := uuid.NewUUID()
		if err != nil {
			return fields, err
		}
		fields.ID = VolumeID.String()
		fields.Dest = volumeStrings[0]
		fields.Flags = "rw"
	case 2:
		fields.ID = volumeStrings[0]
		fields.Dest = volumeStrings[1]
		fields.Flags = "rw"
	case 3:
		fields.ID = volumeStrings[0]
		fields.Dest = volumeStrings[1]
		fields.Flags = volumeStrings[2]
	default:
		// NOTE: the docker cli should cover this case. This is here for posterity.
		return volumeFields{}, errors.InvalidBindError{Volume: volString}
	}
	return fields, nil
}

// processVolumeFields parses fields for volume mappings specified in a create/run -v.
// It returns a map of unique mountable volumes. This means that it removes dupes favoring
// specified volumes over anonymous volumes.
func processVolumeFields(volumes []string) (map[string]volumeFields, error) {
	volumeFields := make(map[string]volumeFields)

	for _, v := range volumes {
		fields, err := processVolumeParam(v)
		log.Infof("Processed volume arguments: %#v", fields)
		if err != nil {
			return nil, err
		}
		volumeFields[fields.Dest] = fields
	}
	return volumeFields, nil
}

func finalizeVolumeList(specifiedVolumes, anonymousVolumes []string) ([]volumeFields, error) {
	log.Infof("Specified Volumes : %#v", specifiedVolumes)
	processedVolumes, err := processVolumeFields(specifiedVolumes)
	if err != nil {
		return nil, err
	}

	log.Infof("anonymous Volumes : %#v", anonymousVolumes)
	processedAnonVolumes, err := processVolumeFields(anonymousVolumes)
	if err != nil {
		return nil, err
	}

	//combine all volumes, specified volumes are taken over anonymous volumes
	for k, v := range processedVolumes {
		processedAnonVolumes[k] = v
	}

	finalizedVolumes := make([]volumeFields, 0, len(processedAnonVolumes))
	for _, v := range processedAnonVolumes {
		finalizedVolumes = append(finalizedVolumes, v)
	}
	return finalizedVolumes, nil
}

func newVolumeCreateReq(name, driverName string, volumeData, labels map[string]string) (*models.VolumeRequest, error) {
	if _, ok := SupportedVolDrivers[driverName]; !ok {
		return nil, fmt.Errorf("error looking up volume plugin %s: plugin not found", driverName)
	}

	if !volumeNameRegex.Match([]byte(name)) && name != "" {
		return nil, fmt.Errorf("volume name %q includes invalid characters, only \"[a-zA-Z0-9][a-zA-Z0-9_.-]\" are allowed", name)
	}

	req := &models.VolumeRequest{
		Driver:     driverName,
		DriverArgs: volumeData,
		Name:       name,
		Metadata:   make(map[string]string),
	}

	metadata, err := createVolumeMetadata(req, volumeData, labels)
	if err != nil {
		return nil, err
	}

	req.Metadata[DockerMetadataModelKey] = metadata

	if err := validateDriverArgs(volumeData, req); err != nil {
		return nil, fmt.Errorf("bad driver value - %s", err)
	}

	return req, nil
}

func validateDriverArgs(args map[string]string, req *models.VolumeRequest) error {
	if err := normalizeDriverArgs(args); err != nil {
		return err
	}

	// volumestore name validation
	req.Store = volumeStore(args)

	// capacity validation
	capstr, ok := args[OptsCapacityKey]
	if !ok {
		req.Capacity = -1
		return nil
	}

	//check if it is just a numerical value
	capacity, err := strconv.ParseInt(capstr, 10, 64)
	if err == nil {
		//input has no units in this case.
		if capacity < 1 {
			return fmt.Errorf("Invalid size: %s", capstr)
		}
		req.Capacity = capacity
		return nil
	}

	capacity, err = units.FromHumanSize(capstr)
	if err != nil {
		return err
	}

	if capacity < 1 {
		return fmt.Errorf("Capacity value too large: %s", capstr)
	}

	req.Capacity = int64(capacity) / int64(units.MB)
	return nil
}

func normalizeDriverArgs(args map[string]string) error {
	// normalize keys to lowercase & validate them
	for k, val := range args {
		lowercase := strings.ToLower(k)

		if _, ok := validDriverOptsKeys[lowercase]; !ok {
			return fmt.Errorf("%s is not a supported option", k)
		}

		if strings.Compare(lowercase, k) != 0 {
			delete(args, k)
			args[lowercase] = val
		}
	}
	return nil
}
