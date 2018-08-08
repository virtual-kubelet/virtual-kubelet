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
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"

	vicfilter "github.com/vmware/vic/lib/apiservers/engine/backends/filter"
	"github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/engine/proxy"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/containers"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/pkg/trace"
)

// Volume which defines the docker personalities view of a Volume
type VolumeBackend struct {
	storageProxy proxy.StorageProxy
}

// acceptedVolumeFilters are volume filters that are supported by VIC
var acceptedVolumeFilters = map[string]bool{
	"dangling": true,
	"name":     true,
	"driver":   true,
	"label":    true,
}

var volumeBackend *VolumeBackend
var volOnce sync.Once

func NewVolumeBackend() *VolumeBackend {
	volOnce.Do(func() {
		volumeBackend = &VolumeBackend{
			storageProxy: proxy.NewStorageProxy(PortLayerClient()),
		}
	})
	return volumeBackend
}

// Volumes docker personality implementation for VIC
func (v *VolumeBackend) Volumes(filter string) ([]*types.Volume, []string, error) {
	op := trace.NewOperation(context.Background(), "Volumes")
	defer trace.End(trace.Audit("", op))

	var volumes []*types.Volume

	// Get volume list from the portlayer
	volumeResponses, err := v.storageProxy.VolumeList(op, filter)
	if err != nil {
		return nil, nil, err
	}

	// Parse and validate filters
	volumeFilters, err := filters.FromParam(filter)
	if err != nil {
		return nil, nil, errors.VolumeInternalServerError(err)
	}
	volFilterContext, err := vicfilter.ValidateVolumeFilters(volumeFilters, acceptedVolumeFilters, nil)
	if err != nil {
		return nil, nil, errors.VolumeInternalServerError(err)
	}

	// joinedVolumes stores names of volumes that are joined to a container
	// and is used while filtering the output by dangling (dangling=true should
	// return volumes that are not attached to a container)
	joinedVolumes := make(map[string]struct{})
	if volumeFilters.Include("dangling") {
		// If the dangling filter is specified, gather required items beforehand
		joinedVolumes, err = fetchJoinedVolumes(op)
		if err != nil {
			return nil, nil, errors.VolumeInternalServerError(err)
		}
	}

	op.Info("volumes found:")
	for _, vol := range volumeResponses {
		op.Infof("%s", vol.Name)

		volumeMetadata, err := extractDockerMetadata(vol.Metadata)
		if err != nil {
			return nil, nil, errors.VolumeInternalServerError(fmt.Errorf("error unmarshalling docker metadata: %s", err))
		}

		// Set fields needed for filtering the output
		volFilterContext.Name = vol.Name
		volFilterContext.Driver = vol.Driver
		_, volFilterContext.Joined = joinedVolumes[vol.Name]
		volFilterContext.Labels = volumeMetadata.Labels

		// Include the volume in the output if it meets the filtering criteria
		filterAction := vicfilter.IncludeVolume(volumeFilters, volFilterContext)
		if filterAction == vicfilter.IncludeAction {
			volume := NewVolumeModel(vol, volumeMetadata.Labels)
			volumes = append(volumes, volume)
		}
	}

	return volumes, nil, nil
}

// VolumeInspect : docker personality implementation for VIC
func (v *VolumeBackend) VolumeInspect(name string) (*types.Volume, error) {
	op := trace.NewOperation(context.Background(), "VolumeInspect: %s", name)
	defer trace.End(trace.Audit(name, op))

	volInfo, err := v.storageProxy.VolumeInfo(op, name)
	if err != nil {
		return nil, err
	}

	volumeMetadata, err := extractDockerMetadata(volInfo.Metadata)
	if err != nil {
		return nil, errors.VolumeInternalServerError(fmt.Errorf("error unmarshalling docker metadata: %s", err))
	}
	volume := NewVolumeModel(volInfo, volumeMetadata.Labels)

	return volume, nil
}

// VolumeCreate : docker personality implementation for VIC
func (v *VolumeBackend) VolumeCreate(name, driverName string, volumeData, labels map[string]string) (*types.Volume, error) {
	op := trace.NewOperation(context.Background(), "VolumeCreate: %s", name)
	defer trace.End(trace.Audit(name, op))

	result, err := v.storageProxy.Create(op, name, driverName, volumeData, labels)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// VolumeRm : docker personality for VIC
func (v *VolumeBackend) VolumeRm(name string, force bool) error {
	op := trace.NewOperation(context.Background(), "VolumeRm: %s", name)
	defer trace.End(trace.Audit(name, op))

	err := v.storageProxy.Remove(op, name)
	if err != nil {
		return err
	}

	return nil
}

func (v *VolumeBackend) VolumesPrune(pruneFilters filters.Args) (*types.VolumesPruneReport, error) {
	op := trace.NewOperation(context.Background(), "VolumesPrune")
	defer trace.End(trace.Audit("", op))

	return nil, errors.APINotSupportedMsg(ProductName(), "VolumesPrune")
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

// fetchJoinedVolumes obtains all containers from the portlayer and returns a map with all
// volumes that are joined to at least one container.
func fetchJoinedVolumes(op trace.Operation) (map[string]struct{}, error) {
	conts, err := allContainers(op)
	if err != nil {
		return nil, errors.VolumeInternalServerError(err)
	}

	joinedVolumes := make(map[string]struct{})
	var s struct{}
	for i := range conts {
		for _, vol := range conts[i].VolumeConfig {
			joinedVolumes[vol.Name] = s
		}
	}

	return joinedVolumes, nil
}

// allContainers obtains all containers from the portlayer, akin to `docker ps -a`.
func allContainers(op trace.Operation) ([]*models.ContainerInfo, error) {
	client := PortLayerClient()
	if client == nil {
		return nil, errors.NillPortlayerClientError("Volume Backend")
	}

	all := true
	cons, err := client.Containers.GetContainerList(containers.NewGetContainerListParamsWithContext(op).WithAll(&all))
	if err != nil {
		return nil, err
	}

	return cons.Payload, nil
}

// Unmarshal the docker metadata using the docker metadata key.  The docker
// metadatakey.  We stash the vals we know about in that map with that key.
func extractDockerMetadata(metadataMap map[string]string) (*proxy.VolumeMetadata, error) {
	v, ok := metadataMap[proxy.DockerMetadataModelKey]
	if !ok {
		return nil, fmt.Errorf("metadata %s missing", proxy.DockerMetadataModelKey)
	}

	result := &proxy.VolumeMetadata{}
	err := json.Unmarshal([]byte(v), result)
	return result, err
}
