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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	derr "github.com/docker/docker/api/errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	dnetwork "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/reference"
	"github.com/docker/go-connections/nat"
	"github.com/go-openapi/runtime"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	viccontainer "github.com/vmware/vic/lib/apiservers/engine/backends/container"
	"github.com/vmware/vic/lib/apiservers/engine/backends/convert"
	"github.com/vmware/vic/lib/apiservers/engine/network"
	"github.com/vmware/vic/lib/apiservers/engine/proxy"
	plclient "github.com/vmware/vic/lib/apiservers/portlayer/client"
	plscopes "github.com/vmware/vic/lib/apiservers/portlayer/client/scopes"
	plmodels "github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/metadata"
)

//***********
// Mock proxy
//***********

type CreateHandleMockData struct {
	createInputID   string
	retID           string
	retHandle       string
	retErr          error
	createErrSubstr string
}

type AddToScopeMockData struct {
	createInputID   string
	retHandle       string
	retErr          error
	createErrSubstr string
}

type AddVolumesMockData struct {
	retHandle       string
	retErr          error
	createErrSubstr string
}

type AddInteractionMockData struct {
	retHandle       string
	retErr          error
	createErrSubstr string
}

type AddLoggingMockData struct {
	retHandle       string
	retErr          error
	createErrSubstr string
}

type CommitHandleMockData struct {
	createInputID   string
	createErrSubstr string

	retErr error
}

type LogMockData struct {
	continaerID string
	running     bool
}

type MockContainerProxy struct {
	mockRespIndices        []int
	mockCreateHandleData   []CreateHandleMockData
	mockAddToScopeData     []AddToScopeMockData
	mockAddVolumesData     []AddVolumesMockData
	mockAddInteractionData []AddInteractionMockData
	mockAddLoggingData     []AddLoggingMockData
	mockCommitData         []CommitHandleMockData
}

type MockStorageProxy struct {
}

type MockStreamProxy struct {
}

const (
	SUCCESS             = 0
	dummyContainerID    = "abc123"
	dummyContainerIDTTY = "tty123"
	fakeContainerID     = ""
)

var randomNames = []string{
	"hello_world",
	"hello_world",
	"goodbye_world",
	"goodbye_world",
	"cruel_world",
}

func mockRandomName(retry int) string {
	return randomNames[retry%len(randomNames)]
}

var dummyContainers = []string{dummyContainerID, dummyContainerIDTTY}

func NewMockContainerProxy() *MockContainerProxy {
	return &MockContainerProxy{
		mockRespIndices:        make([]int, 6),
		mockCreateHandleData:   MockCreateHandleData(),
		mockAddToScopeData:     MockAddToScopeData(),
		mockAddVolumesData:     MockAddVolumesData(),
		mockAddInteractionData: MockAddInteractionData(),
		mockAddLoggingData:     MockAddLoggingData(),
		mockCommitData:         MockCommitData(),
	}
}

func NewMockStorageProxy() *MockStorageProxy {
	return &MockStorageProxy{}
}

func NewMockStreamProxy() *MockStreamProxy {
	return &MockStreamProxy{}
}

func MockCreateHandleData() []CreateHandleMockData {

	createHandleTimeoutErr := runtime.NewAPIError("unknown error", "context deadline exceeded", http.StatusServiceUnavailable)

	mockCreateHandleData := []CreateHandleMockData{
		{"busybox", "321cba", "handle", nil, ""},
		{"busybox", "", "", derr.NewRequestNotFoundError(fmt.Errorf("No such image: abc123")), "No such image"},
		{"busybox", "", "", derr.NewErrorWithStatusCode(createHandleTimeoutErr, http.StatusInternalServerError), "context deadline exceeded"},
	}

	return mockCreateHandleData
}

func MockAddToScopeData() []AddToScopeMockData {
	addToScopeNotFound := plscopes.AddContainerNotFound{
		Payload: &plmodels.Error{
			Message: "Scope not found",
		},
	}

	addToScopeNotFoundErr := fmt.Errorf("ContainerProxy.AddContainerToScope: Scopes error: %s", addToScopeNotFound.Error())

	addToScopeTimeout := plscopes.AddContainerInternalServerError{
		Payload: &plmodels.Error{
			Message: "context deadline exceeded",
		},
	}

	addToScopeTimeoutErr := fmt.Errorf("ContainerProxy.AddContainerToScope: Scopes error: %s", addToScopeTimeout.Error())

	mockAddToScopeData := []AddToScopeMockData{
		{"busybox", "handle", nil, ""},
		{"busybox", "handle", derr.NewErrorWithStatusCode(fmt.Errorf("container.ContainerCreate failed to create a portlayer client"), http.StatusInternalServerError), "failed to create a portlayer"},
		{"busybox", "handle", derr.NewErrorWithStatusCode(addToScopeNotFoundErr, http.StatusInternalServerError), "Scope not found"},
		{"busybox", "handle", derr.NewErrorWithStatusCode(addToScopeTimeoutErr, http.StatusInternalServerError), "context deadline exceeded"},
	}

	return mockAddToScopeData
}

func MockAddVolumesData() []AddVolumesMockData {
	return nil
}

func MockAddInteractionData() []AddInteractionMockData {
	return nil
}

func MockAddLoggingData() []AddLoggingMockData {
	return nil
}

func MockCommitData() []CommitHandleMockData {
	noSuchImageErr := fmt.Errorf("No such image: busybox")

	mockCommitData := []CommitHandleMockData{
		{"buxybox", "", nil},
		{"busybox", "failed to create a portlayer", derr.NewErrorWithStatusCode(fmt.Errorf("container.ContainerCreate failed to create a portlayer client"), http.StatusInternalServerError)},
		{"busybox", "No such image", derr.NewRequestNotFoundError(noSuchImageErr)},
	}

	return mockCommitData
}

func (m *MockContainerProxy) GetMockDataCount() (int, int, int, int) {
	return len(m.mockCreateHandleData), len(m.mockAddToScopeData), len(m.mockAddVolumesData), len(m.mockCommitData)
}

func (m *MockContainerProxy) SetMockDataResponse(createHandleResp int, addToScopeResp int, addVolumeResp int, addInteractionResp int, addLoggingResp int, commitContainerResp int) {
	m.mockRespIndices[0] = createHandleResp
	m.mockRespIndices[1] = addToScopeResp
	m.mockRespIndices[2] = addVolumeResp
	m.mockRespIndices[3] = addInteractionResp
	m.mockRespIndices[4] = addLoggingResp
	m.mockRespIndices[5] = commitContainerResp
}

func (m *MockContainerProxy) Handle(ctx context.Context, id, name string) (string, error) {
	return "", nil
}

func (m *MockContainerProxy) CreateContainerHandle(ctx context.Context, vc *viccontainer.VicContainer, config types.ContainerCreateConfig) (string, string, error) {
	respIdx := m.mockRespIndices[0]

	if respIdx >= len(m.mockCreateHandleData) {
		return "", "", nil
	}
	return m.mockCreateHandleData[respIdx].retID, m.mockCreateHandleData[respIdx].retHandle, m.mockCreateHandleData[respIdx].retErr
}

func (m *MockContainerProxy) CreateContainerTask(ctx context.Context, handle string, id string, config types.ContainerCreateConfig) (string, error) {
	respIdx := m.mockRespIndices[0]

	if respIdx >= len(m.mockCreateHandleData) {
		return "", nil
	}
	return m.mockCreateHandleData[respIdx].retHandle, m.mockCreateHandleData[respIdx].retErr
}

func (m *MockContainerProxy) AddContainerToScope(ctx context.Context, handle string, config types.ContainerCreateConfig) (string, error) {
	respIdx := m.mockRespIndices[1]

	if respIdx >= len(m.mockAddToScopeData) {
		return "", nil
	}

	return m.mockAddToScopeData[respIdx].retHandle, m.mockAddToScopeData[respIdx].retErr
}

func (m *MockContainerProxy) AddVolumesToContainer(ctx context.Context, handle string, config types.ContainerCreateConfig) (string, error) {
	respIdx := m.mockRespIndices[2]

	if respIdx >= len(m.mockAddVolumesData) {
		return "", nil
	}

	return m.mockAddVolumesData[respIdx].retHandle, m.mockAddVolumesData[respIdx].retErr
}

func (m *MockContainerProxy) AddInteractionToContainer(ctx context.Context, handle string, config types.ContainerCreateConfig) (string, error) {
	respIdx := m.mockRespIndices[3]

	if respIdx >= len(m.mockAddInteractionData) {
		return "", nil
	}

	return m.mockAddInteractionData[respIdx].retHandle, m.mockAddInteractionData[respIdx].retErr
}

func (m *MockContainerProxy) AddLoggingToContainer(ctx context.Context, handle string, config types.ContainerCreateConfig) (string, error) {
	respIdx := m.mockRespIndices[4]

	if respIdx >= len(m.mockAddLoggingData) {
		return "", nil
	}

	return m.mockAddLoggingData[respIdx].retHandle, m.mockAddLoggingData[respIdx].retErr
}

func (m *MockContainerProxy) BindInteraction(ctx context.Context, handle string, name string, id string) (string, error) {
	return "", nil
}

func (m *MockContainerProxy) CreateExecTask(ctx context.Context, handle string, config *types.ExecConfig) (string, string, error) {
	return "", "", nil
}

func (m *MockContainerProxy) UnbindInteraction(ctx context.Context, handle string, name string, id string) (string, error) {
	return "", nil
}

func (m *MockContainerProxy) CommitContainerHandle(ctx context.Context, handle, containerID string, waitTime int32) error {
	respIdx := m.mockRespIndices[5]

	if respIdx >= len(m.mockCommitData) {
		return nil
	}

	return m.mockCommitData[respIdx].retErr
}

func (m *MockContainerProxy) Client() *plclient.PortLayer {
	return nil
}

func (m *MockContainerProxy) Stop(ctx context.Context, vc *viccontainer.VicContainer, name string, seconds *int, unbound bool) error {
	return nil
}

func (m *MockContainerProxy) State(ctx context.Context, vc *viccontainer.VicContainer) (*types.ContainerState, error) {
	// Assume container is running if container in cache.  If we need other conditions
	// in the future, we can add it, but for now, just assume running.
	c := cache.ContainerCache().GetContainer(vc.ContainerID)

	if c == nil {
		return nil, nil
	}

	state := &types.ContainerState{
		Running: true,
	}
	return state, nil
}

func (m *MockContainerProxy) Wait(ctx context.Context, vc *viccontainer.VicContainer, timeout time.Duration) (*types.ContainerState, error) {
	dockerState := &types.ContainerState{ExitCode: 0}
	return dockerState, nil
}

func (m *MockContainerProxy) Signal(ctx context.Context, vc *viccontainer.VicContainer, sig uint64) error {
	return nil
}

func (m *MockContainerProxy) Resize(ctx context.Context, id string, height, width int32) error {
	return nil
}

func (m *MockContainerProxy) Rename(ctx context.Context, vc *viccontainer.VicContainer, newName string) error {
	return nil
}

func (m *MockContainerProxy) Remove(ctx context.Context, vc *viccontainer.VicContainer, config *types.ContainerRmConfig) error {
	return nil
}

func (m *MockContainerProxy) StreamContainerStats(ctx context.Context, config *convert.ContainerStatsConfig) error {
	return nil
}

func (m *MockContainerProxy) UnbindContainerFromNetwork(ctx context.Context, vc *viccontainer.VicContainer, handle string) (string, error) {
	return "", nil
}

func (m *MockContainerProxy) ExitCode(ctx context.Context, vc *viccontainer.VicContainer) (string, error) {
	return "", nil
}

func AddMockImageToCache() {
	mockImage := &metadata.ImageConfig{
		ImageID:   "e732471cb81a564575aad46b9510161c5945deaf18e9be3db344333d72f0b4b2",
		Name:      "busybox",
		Tags:      []string{"latest"},
		Reference: "busybox:latest",
	}
	mockImage.Config = &container.Config{
		Hostname:     "55cd1f8f6e5b",
		Domainname:   "",
		User:         "",
		AttachStdin:  false,
		AttachStdout: false,
		AttachStderr: false,
		Tty:          false,
		OpenStdin:    false,
		StdinOnce:    false,
		Env:          []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		Cmd:          []string{"sh"},
		Image:        "sha256:e732471cb81a564575aad46b9510161c5945deaf18e9be3db344333d72f0b4b2",
		Volumes:      nil,
		WorkingDir:   "",
		Entrypoint:   nil,
		OnBuild:      nil,
	}

	cache.ImageCache().Add(mockImage)

	ref, _ := reference.ParseNamed(mockImage.Reference)
	cache.RepositoryCache().AddReference(ref, mockImage.ImageID, false, mockImage.ImageID, false)
}

func AddMockContainerToCache() {
	AddMockImageToCache()

	image, err := cache.ImageCache().Get("e732471cb81a564575aad46b9510161c5945deaf18e9be3db344333d72f0b4b2")
	if err == nil {
		vc := viccontainer.NewVicContainer()
		vc.ImageID = image.ID
		vc.Config = image.Config //Set defaults.  Overrides will get copied below.
		vc.Config.Tty = false
		vc.ContainerID = dummyContainerID
		cache.ContainerCache().AddContainer(vc)

		vc = viccontainer.NewVicContainer()
		vc.ImageID = image.ID
		vc.Config = image.Config
		vc.Config.Tty = true
		vc.ContainerID = dummyContainerIDTTY
		cache.ContainerCache().AddContainer(vc)

		vc = viccontainer.NewVicContainer()
		vc.ImageID = image.ID
		vc.Config = image.Config
		vc.Config.Tty = false
		vc.ContainerID = fakeContainerID
		cache.ContainerCache().AddContainer(vc)
	}
}

func (s *MockStorageProxy) Create(ctx context.Context, name, driverName string, volumeData, labels map[string]string) (*types.Volume, error) {
	return nil, nil
}

func (s *MockStorageProxy) VolumeList(ctx context.Context, filter string) ([]*plmodels.VolumeResponse, error) {
	return nil, nil
}

func (s *MockStorageProxy) VolumeInfo(ctx context.Context, name string) (*plmodels.VolumeResponse, error) {
	return nil, nil
}

func (s *MockStorageProxy) Remove(ctx context.Context, name string) error {
	return nil
}

func (s *MockStorageProxy) AddVolumesToContainer(ctx context.Context, handle string, config types.ContainerCreateConfig) (string, error) {
	return "", nil
}

func (sp *MockStreamProxy) AttachStreams(ctx context.Context, ac *proxy.AttachConfig, stdin io.ReadCloser, stdout, stderr io.Writer) error {
	return nil
}

func (sp *MockStreamProxy) StreamContainerLogs(_ context.Context, name string, out io.Writer, started chan struct{}, showTimestamps bool, followLogs bool, since int64, tailLines int64) error {
	if name == "" {
		return fmt.Errorf("sample error message")
	}

	var lineCount int64 = 10

	close(started)

	for i := int64(0); i < lineCount; i++ {
		if !followLogs && i > tailLines {
			break
		}
		if followLogs && i > tailLines {
			time.Sleep(500 * time.Millisecond)
		}

		fmt.Fprintf(out, "line %d\n", i)
	}

	return nil
}

func (sp *MockStreamProxy) StreamContainerStats(ctx context.Context, config *convert.ContainerStatsConfig) error {
	return nil
}

//***********
// Tests
//***********

// TestContainerCreateEmptyImageCache() attempts a ContainerCreate() with an empty image
// cache
func TestContainerCreateEmptyImageCache(t *testing.T) {
	mockContainerProxy := NewMockContainerProxy()

	// Create our personality Container backend
	cb := &ContainerBackend{
		containerProxy: mockContainerProxy,
	}

	// mock a container create config
	var config types.ContainerCreateConfig

	config.HostConfig = &container.HostConfig{}
	config.Config = &container.Config{}
	config.NetworkingConfig = &dnetwork.NetworkingConfig{}
	config.Config.Image = "busybox"

	_, err := cb.ContainerCreate(config)

	assert.Contains(t, err.Error(), "No such image", "Error (%s) should have 'No such image' for an empty image cache", err.Error())
}

// TestCreateHandle() cycles through all possible input/outputs for creating a handle
// and calls vicbackends.ContainerCreate().  The idea is that if creating handle fails
// then vicbackends.ContainerCreate() should return errors from that.
func TestCreateHandle(t *testing.T) {
	mockContainerProxy := NewMockContainerProxy()

	// Create our personality Container backend
	cb := &ContainerBackend{
		containerProxy: mockContainerProxy,
	}

	AddMockImageToCache()

	// configure mock naming for just this test
	defer func(fn func(int) string) {
		randomName = fn
	}(randomName)
	randomName = mockRandomName

	// mock a container create config
	var config types.ContainerCreateConfig

	config.HostConfig = &container.HostConfig{}
	config.Config = &container.Config{}
	config.NetworkingConfig = &dnetwork.NetworkingConfig{}

	mockCreateHandleData := MockCreateHandleData()

	// Iterate over create handler responses and see what the composite ContainerCreate()
	// returns.  Since the handle is the first operation, we expect to receive a create handle
	// error.
	count, _, _, _ := mockContainerProxy.GetMockDataCount()

	for i := 0; i < count; i++ {
		if i == SUCCESS { //skip success case
			continue
		}

		mockContainerProxy.SetMockDataResponse(i, 0, 0, 0, 0, 0)
		config.Config.Image = mockCreateHandleData[i].createInputID
		_, err := cb.ContainerCreate(config)

		assert.Contains(t, err.Error(), mockCreateHandleData[i].createErrSubstr)
	}
}

// TestContainerAddToScope() assumes container handle create succeeded and cycles through all
// possible input/outputs for adding container to scope and calls vicbackends.ContainerCreate()
func TestContainerAddToScope(t *testing.T) {
	mockContainerProxy := NewMockContainerProxy()

	// Create our personality Container backend
	cb := &ContainerBackend{
		containerProxy: mockContainerProxy,
	}

	AddMockImageToCache()

	// mock a container create config
	var config types.ContainerCreateConfig

	config.HostConfig = &container.HostConfig{}
	config.Config = &container.Config{}
	config.NetworkingConfig = &dnetwork.NetworkingConfig{}

	mockAddToScopeData := MockAddToScopeData()

	// Iterate over create handler responses and see what the composite ContainerCreate()
	// returns.  Since the handle is the first operation, we expect to receive a create handle
	// error.
	_, count, _, _ := mockContainerProxy.GetMockDataCount()

	for i := 0; i < count; i++ {
		if i == SUCCESS { //skip success case
			continue
		}

		mockContainerProxy.SetMockDataResponse(0, i, 0, 0, 0, 0)
		config.Config.Image = mockAddToScopeData[i].createInputID
		_, err := cb.ContainerCreate(config)

		assert.Contains(t, err.Error(), mockAddToScopeData[i].createErrSubstr)
	}
}

// TestContainerAddVolumes() assumes container handle create succeeded and cycles through all
// possible input/outputs for committing the handle and calls vicbackends.ContainerCreate()
func TestCommitHandle(t *testing.T) {
	mockContainerProxy := NewMockContainerProxy()
	mockStorageProxy := NewMockStorageProxy()

	// Create our personality Container backend
	cb := &ContainerBackend{
		containerProxy: mockContainerProxy,
		storageProxy:   mockStorageProxy,
	}

	AddMockImageToCache()

	// mock a container create config
	var config types.ContainerCreateConfig

	config.HostConfig = &container.HostConfig{}
	config.Config = &container.Config{}
	config.NetworkingConfig = &dnetwork.NetworkingConfig{}

	mockCommitHandleData := MockCommitData()

	// Iterate over create handler responses and see what the composite ContainerCreate()
	// returns.  Since the handle is the first operation, we expect to receive a create handle
	// error.
	_, _, _, count := mockContainerProxy.GetMockDataCount()

	for i := 0; i < count; i++ {
		if i == SUCCESS { //skip success case
			continue
		}

		mockContainerProxy.SetMockDataResponse(0, 0, 0, 0, 0, i)
		config.Config.Image = mockCommitHandleData[i].createInputID
		_, err := cb.ContainerCreate(config)

		assert.Contains(t, err.Error(), mockCommitHandleData[i].createErrSubstr)
	}

}

// TestContainerLogs() tests the docker logs api when user asks for entire log
func TestContainerLogs(t *testing.T) {
	// Create our personality Container backend
	cb := &ContainerBackend{
		containerProxy: NewMockContainerProxy(),
		streamProxy:    NewMockStreamProxy(),
	}

	// Prepopulate our image and container cache with dummy data
	AddMockContainerToCache()

	// Create a buffer io.writer
	var writer bytes.Buffer

	successDuration := 1 * time.Second

	// Create our mock table
	mockData := []struct {
		Config          backend.ContainerLogsConfig
		ExpectedSuccess bool
		ExpectedFollow  bool
	}{
		{
			Config: backend.ContainerLogsConfig{
				ContainerLogsOptions: types.ContainerLogsOptions{
					ShowStdout: true,
					ShowStderr: true,
					Tail:       "all",
				},
				OutStream: &writer,
			},
			ExpectedSuccess: true,
			ExpectedFollow:  false,
		},
		{
			Config: backend.ContainerLogsConfig{
				ContainerLogsOptions: types.ContainerLogsOptions{
					ShowStdout: false,
					ShowStderr: false,
				},
				OutStream: &writer,
			},
			ExpectedSuccess: false,
			ExpectedFollow:  false,
		},
		{
			Config: backend.ContainerLogsConfig{
				ContainerLogsOptions: types.ContainerLogsOptions{
					ShowStdout: true,
					ShowStderr: true,
					Follow:     true,
				},
				OutStream: &writer,
			},
			ExpectedSuccess: true,
			ExpectedFollow:  true,
		},
	}

	for _, containerID := range dummyContainers {
		for _, data := range mockData {
			started := make(chan struct{})

			start := time.Now()
			err := cb.ContainerLogs(context.TODO(), containerID, &data.Config, started)
			end := time.Now()

			select {
			case <-started:
			default:
				close(started)
			}

			if data.ExpectedSuccess {
				assert.Nil(t, err, "Expected success, but got error, config: %#v", data.Config)
			} else {
				assert.NotEqual(t, err, nil, "Expected error but received nil, config: %#v", data.Config)
			}

			immediate := start.Add(successDuration)

			didFollow := immediate.Before(end) //determines if logs continued to stream

			if data.ExpectedFollow {
				assert.True(t, didFollow, "Expected logs to follow but didn't (%s, %s), config: %#v", start.String(), end.String(), data.Config)
			} else {
				assert.False(t, didFollow, "Expected logs to NOT follow but it did, config: %#v", data.Config)
			}
		}
	}

	// Check that ContainerLogs *does not* return an error if StreamContainerLogs
	// returns an error. Here, the config is valid and the container is in the
	// cache, so the only error will come from StreamContainerLogs. Since the
	// containerID = "", StreamContainerLogs will return an error.
	started := make(chan struct{})
	err := cb.ContainerLogs(context.TODO(), fakeContainerID, &mockData[0].Config, started)
	assert.NoError(t, err)
}

func TestPortInformation(t *testing.T) {
	mockContainerInfo := &plmodels.ContainerInfo{}
	mockContainerConfig := &plmodels.ContainerConfig{}
	containerID := "foo"
	mockContainerConfig.ContainerID = containerID

	mockHostConfig := &container.HostConfig{}

	portMap := nat.PortMap{}
	port, _ := nat.NewPort("tcp", "80")
	portBinding := nat.PortBinding{
		HostIP:   "127.0.0.1",
		HostPort: "8000",
	}
	portBindings := []nat.PortBinding{portBinding}
	portMap[port] = portBindings
	mockHostConfig.PortBindings = portMap

	mockContainerInfo.ContainerConfig = mockContainerConfig
	mockContainerInfo.Endpoints = []*plmodels.EndpointConfig{
		{
			Direct: true,
			Trust:  executor.Published.String(),
			Ports:  []string{"8000/tcp"},
		},
	}

	ips := []string{"192.168.1.1"}

	co := viccontainer.NewVicContainer()
	co.HostConfig = mockHostConfig
	co.NATMap = portMap
	co.ContainerID = containerID
	co.Name = "bar"
	cache.ContainerCache().AddContainer(co)

	// unless there are entries in vicnetwork.ContainerByPort we won't report them as bound
	ports := network.PortForwardingInformation(co, ips)
	assert.Empty(t, ports, "There should be no bound IPs at this point for forwarding")

	// the current port binding should show up as a direct port
	ports = network.DirectPortInformation(mockContainerInfo)
	assert.NotEmpty(t, ports, "There should be a direct port")

	network.ContainerByPort["8000"] = containerID
	ports = network.PortForwardingInformation(co, ips)
	assert.NotEmpty(t, ports, "There should be bound IPs")
	assert.Equal(t, 1, len(ports), "Expected 1 port binding, found %d", len(ports))
	// now that this port presents as a forwarded port it should NOT present as a direct port
	ports = network.DirectPortInformation(mockContainerInfo)
	assert.Empty(t, ports, "There should not be a direct port")

	port, _ = nat.NewPort("tcp", "80")
	portBinding = nat.PortBinding{
		HostIP:   "127.0.0.1",
		HostPort: "00",
	}
	portMap[port] = portBindings

	// forwarding of 00 should never happen, but this is allowing us to confirm that
	// it's kicked out by the function even if present in the map
	network.ContainerByPort["00"] = containerID
	ports = network.PortForwardingInformation(co, ips)
	assert.NotEmpty(t, ports, "There should be 1 bound IP")
	assert.Equal(t, 1, len(ports), "Expected 1 port binding, found %d", len(ports))

	port, _ = nat.NewPort("tcp", "800")
	portBinding = nat.PortBinding{
		HostIP:   "127.0.0.1",
		HostPort: "800",
	}
	portMap[port] = portBindings
	network.ContainerByPort["800"] = containerID
	ports = network.PortForwardingInformation(co, ips)
	assert.Equal(t, 2, len(ports), "Expected 2 port binding, found %d", len(ports))
}

// TestCreateConfigNetowrkMode() whether the HostConfig.NetworkMode is set correctly in ValidateCreateConfig()
func TestCreateConfigNetworkMode(t *testing.T) {

	// mock a container create config
	mockConfig := types.ContainerCreateConfig{
		HostConfig: &container.HostConfig{},
		Config: &container.Config{
			Image: "busybox",
		},
		NetworkingConfig: &dnetwork.NetworkingConfig{
			EndpointsConfig: map[string]*dnetwork.EndpointSettings{
				"net1": {},
			},
		},
	}

	validateCreateConfig(&mockConfig)

	assert.Equal(t, mockConfig.HostConfig.NetworkMode.NetworkName(), "net1", "expected NetworkMode is net1, found %s", mockConfig.HostConfig.NetworkMode)

	// container connects to two vicnetwork endpoints; check for NetworkMode error
	mockConfig.NetworkingConfig.EndpointsConfig["net2"] = &dnetwork.EndpointSettings{}

	err := validateCreateConfig(&mockConfig)

	assert.Contains(t, err.Error(), "NetworkMode error", "error (%s) should have 'NetworkMode error'", err.Error())
}
