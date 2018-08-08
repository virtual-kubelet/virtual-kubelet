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

package client

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/docker/docker/opts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/vmware/govmomi/list"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/config"
	lib_executor "github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/vchlog"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) CreateVCH(conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData, receiver vchlog.Receiver) error {
	args := m.Called(conf, settings, receiver)

	return args.Error(0)
}

func (m *MockExecutor) DeleteVCH(conf *config.VirtualContainerHostConfigSpec, containers *management.DeleteContainers, volumeStores *management.DeleteVolumeStores) error {
	args := m.Called(conf, containers, volumeStores)

	return args.Error(0)
}

func (m *MockExecutor) NewVCHFromID(id string) (*vm.VirtualMachine, error) {
	args := m.Called(id)

	a := args.Get(0)
	if a == nil {
		return nil, args.Error(1)
	}

	return a.(*vm.VirtualMachine), args.Error(1)
}

func (m *MockExecutor) SearchVCHs(computePath string) ([]*vm.VirtualMachine, error) {
	args := m.Called(computePath)

	a := args.Get(0)
	if a == nil {
		return nil, args.Error(1)
	}

	return a.([]*vm.VirtualMachine), args.Error(1)
}

func (m *MockExecutor) GetNoSecretVCHConfig(vm *vm.VirtualMachine) (*config.VirtualContainerHostConfigSpec, error) {
	args := m.Called(vm)

	a := args.Get(0)
	if a == nil {
		return nil, args.Error(1)
	}

	return a.(*config.VirtualContainerHostConfigSpec), args.Error(1)
}

func (m *MockExecutor) GetTLSFriendlyHostIP(clientIP net.IP, cert *x509.Certificate, certificateAuthorities []byte) string {
	args := m.Called(clientIP, cert, certificateAuthorities)

	return args.String(0)
}

type MockFinder struct {
	mock.Mock
}

func (m *MockFinder) Element(ctx context.Context, ref types.ManagedObjectReference) (*list.Element, error) {
	args := m.Called(ctx, ref)

	a := args.Get(0)
	if a == nil {
		return nil, args.Error(1)
	}

	return a.(*list.Element), args.Error(1)
}

func (m *MockFinder) Datastore(ctx context.Context, path string) (*object.Datastore, error) {
	args := m.Called(ctx, path)

	a := args.Get(0)
	if a == nil {
		return nil, args.Error(1)
	}

	return a.(*object.Datastore), args.Error(1)
}

func (m *MockFinder) ObjectReference(ctx context.Context, ref types.ManagedObjectReference) (object.Reference, error) {
	args := m.Called(ctx, ref)

	a := args.Get(0)
	if a == nil {
		return nil, args.Error(1)
	}

	return a.(object.Reference), args.Error(1)
}

type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) AddDeprecatedFields(ctx context.Context, conf *config.VirtualContainerHostConfigSpec, input *data.Data) *data.InstallerData {
	args := m.Called(ctx, conf, input)

	a := args.Get(0)
	if a == nil {
		return nil
	}

	return a.(*data.InstallerData)
}

func (m *MockValidator) GetIssues() []error {
	args := m.Called()

	a := args.Get(0)
	if a == nil {
		return nil
	}

	return a.([]error)
}

func (m *MockValidator) Validate(ctx context.Context, input *data.Data, allowEmptyDC bool) (*config.VirtualContainerHostConfigSpec, error) {
	args := m.Called(ctx, input, allowEmptyDC)

	a := args.Get(0)
	if a == nil {
		return nil, args.Error(1)
	}

	return a.(*config.VirtualContainerHostConfigSpec), args.Error(1)
}

func (m *MockValidator) SetDataFromVM(ctx context.Context, vm *vm.VirtualMachine, d *data.Data) error {
	args := m.Called(ctx, vm, d)

	return args.Error(0)
}

func makeMocks() (hc HandlerClient, me *MockExecutor, mf *MockFinder, mv *MockValidator) {
	me = &MockExecutor{}
	mf = &MockFinder{}
	s := &session.Session{Config: &session.Config{}}
	mv = &MockValidator{}

	hc = HandlerClient{
		executor:  me,
		finder:    mf,
		session:   s,
		validator: mv,
	}

	return
}

func checkMocks(t *testing.T, me *MockExecutor, mf *MockFinder, mv *MockValidator) {
	me.AssertExpectations(t)
	mf.AssertExpectations(t)
	mv.AssertExpectations(t)
}

type statusCode interface {
	Code() int
}

func assert404(t *testing.T, err error) {
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), errFake.Error())
	assert.Equal(t, err.(statusCode).Code(), http.StatusNotFound)
}

func assert500(t *testing.T, err error) {
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), errFake.Error())
	assert.Equal(t, err.(statusCode).Code(), http.StatusInternalServerError)
}

var (
	fakeOp     = trace.NewOperation(context.Background(), "Test Operation")
	fakeData   = &data.Data{VCHID: common.VCHID{ID: "VM ID"}}
	fakeVM     = &vm.VirtualMachine{}
	fakeConfig = &config.VirtualContainerHostConfigSpec{}
	errFake    = fmt.Errorf("Expected Error")

	fakeIP          = "192.0.2.1"
	fakeHost        = "example.com"
	fakeCertificate = `-----BEGIN CERTIFICATE-----
MIIC6jCCAlOgAwIBAgIJALwtu7/OvhmfMA0GCSqGSIb3DQEBCwUAMIGNMQswCQYD
VQQGEwJVUzETMBEGA1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwNU2FuIEZyYW5j
aXNjbzEVMBMGA1UECgwMVk13YXJlLCBJbmMuMSYwJAYDVQQLDB12U3BoZXJlIElu
dGVncmF0ZWQgQ29udGFpbmVyczESMBAGA1UEAwwJVW5pdCBUZXN0MB4XDTE4MDUw
NDIxMTIzMloXDTE4MDYwMzIxMTIzMlowgY0xCzAJBgNVBAYTAlVTMRMwEQYDVQQI
DApDYWxpZm9ybmlhMRYwFAYDVQQHDA1TYW4gRnJhbmNpc2NvMRUwEwYDVQQKDAxW
TXdhcmUsIEluYy4xJjAkBgNVBAsMHXZTcGhlcmUgSW50ZWdyYXRlZCBDb250YWlu
ZXJzMRIwEAYDVQQDDAlVbml0IFRlc3QwgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJ
AoGBAMgK48u7VBNUD+3cddymD5d3O1jxVxOJYorNbWgzyETO7En2lpDRDrZHk1nH
DNWDo7yvPPEnGmxc/buY2NwxoghCNrwoB80j+rcwGv7vqA/3fnXcdpY4lzv1qPcj
ZOsy6c50+rny4jA8jlXQ7+tU0x3B9UKqHu+MZQrZeeyzjIpjAgMBAAGjUDBOMB0G
A1UdDgQWBBSsWQFtWtIlq9ES0nCCOnt1oX7t8zAfBgNVHSMEGDAWgBSsWQFtWtIl
q9ES0nCCOnt1oX7t8zAMBgNVHRMEBTADAQH/MA0GCSqGSIb3DQEBCwUAA4GBAJ3c
gZ1sQn7FO4WZcIOzYfS/EJDSY4xsPdhHFYWPIa0aau9TJ7STmO9UjOuN5jb2E49M
yw7f/MBHy9cmYQ+jY5UWshesrqFDXHYDmmYz7Qo4RbqS8fFrRasexe1LsIh2ErgM
jYB+p60rlPSezfJP5CQ3t9IP7sYB6TwLDdOfTW6L
-----END CERTIFICATE-----`
	fakeKey, _ = pem.Decode([]byte(`-----BEGIN PRIVATE KEY-----
MIICdwIBADANBgkqhkiG9w0BAQEFAASCAmEwggJdAgEAAoGBAMgK48u7VBNUD+3c
ddymD5d3O1jxVxOJYorNbWgzyETO7En2lpDRDrZHk1nHDNWDo7yvPPEnGmxc/buY
2NwxoghCNrwoB80j+rcwGv7vqA/3fnXcdpY4lzv1qPcjZOsy6c50+rny4jA8jlXQ
7+tU0x3B9UKqHu+MZQrZeeyzjIpjAgMBAAECgYEAoJvtrQMoS6RwbZ9VmeRSHGAE
bDLIoMzrK1on/0OkBWrFV9T9qiPPVhY9fhVMfpkEe1eO7Gdi1aILrfTYGGJZHi5H
8boMWH6bEonT2Kq1Q3Wv+d4IJjRD/UN2FzAZ+8g8AvCZRBGOVsuqclQEdvIPulXL
HalU+Q6mAZKePG8GKYECQQD/ki9WtMoH3sPmzBCUTc8aWBrEQC6vYuSUbgQPe3XW
hVv4iZLEdfdtp8m1LnsbycwCyXmtf2gMsW5j3ZUls0LhAkEAyGDYWwyv5kq1qJ3m
yKfc9MJAIZyzPMA6vGENESmIz6a5yc7sOF2G8bXWHc4N3T7G3ZJi4ePu0bmBvIST
Miy5wwJAZ8UBd6E8jumCfYnKCY12U+oGJD0zN39d9G6fM3IbrJjFeSrS7vY/GsUP
/4L59ZSAQ3lu8GVU6CJ7Ag2Ma5xXwQJAci6FezS6k08lPwVjehn1ld+PLdgeZtLf
ZXMkQBBb7oACRJZOEzxwZhIJBgjh654XMjF1eWUqNIYyAJvHSQMlgwJBAIwHP2hX
S6sCT8jKgNPmt7vaEXFl79Z2lXfrMid86U4zabaxn5WVeAIp0YBGdtWU5c6W7Rtb
guYQ7S7BkqPlWrw=
-----END PRIVATE KEY-----`))
)

func TestHandlerConfig_Executor(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	res := hc.Executor()

	checkMocks(t, me, mf, mv)

	assert.Equal(t, res, me)
}

func TestHandlerConfig_Finder(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	res := hc.Finder()

	checkMocks(t, me, mf, mv)

	assert.Equal(t, res, mf)
}

func TestHandlerConfig_Validator(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	res := hc.Validator()

	checkMocks(t, me, mf, mv)

	assert.Equal(t, res, mv)
}

func TestHandlerConfig_GetVCH(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	me.On("NewVCHFromID", fakeData.ID).Return(fakeVM, nil)
	mv.On("SetDataFromVM", fakeOp, fakeVM, fakeData).Return(nil)

	res, err := hc.GetVCH(fakeOp, fakeData)

	checkMocks(t, me, mf, mv)

	assert.Equal(t, res, fakeVM)
	assert.Nil(t, err)
}

func TestHandlerConfig_GetVCH_LookupError(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	me.On("NewVCHFromID", fakeData.ID).Return(nil, errFake)

	res, err := hc.GetVCH(fakeOp, fakeData)

	checkMocks(t, me, mf, mv)

	assert.Nil(t, res)
	assert404(t, err)
}

func TestHandlerConfig_GetVCH_LoadError(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	me.On("NewVCHFromID", fakeData.ID).Return(fakeVM, nil)
	mv.On("SetDataFromVM", fakeOp, fakeVM, fakeData).Return(errFake)

	res, err := hc.GetVCH(fakeOp, fakeData)

	checkMocks(t, me, mf, mv)

	assert.Nil(t, res)
	assert500(t, err)
}

func TestHandlerConfig_GetVCHs(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	me.On("SearchVCHs", mock.Anything).Return([]*vm.VirtualMachine{fakeVM}, nil)

	res, err := hc.GetVCHs(fakeOp)

	checkMocks(t, me, mf, mv)

	assert.Contains(t, res, fakeVM)
	assert.Nil(t, err)
}

func TestHandlerConfig_GetVCHs_LookupError(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	me.On("SearchVCHs", mock.Anything).Return(nil, errFake)

	res, err := hc.GetVCHs(fakeOp)

	checkMocks(t, me, mf, mv)

	assert.Nil(t, res)
	assert500(t, err)
}

func TestHandlerConfig_GetConfigForVCH(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	me.On("GetNoSecretVCHConfig", fakeVM).Return(fakeConfig, nil)

	res, err := hc.GetConfigForVCH(fakeOp, fakeVM)

	checkMocks(t, me, mf, mv)

	assert.Equal(t, res, fakeConfig)
	assert.Nil(t, err)
}

func TestHandlerConfig_GetConfigForVCH_ConfigLoadError(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	me.On("GetNoSecretVCHConfig", fakeVM).Return(nil, errFake)

	res, err := hc.GetConfigForVCH(fakeOp, fakeVM)

	checkMocks(t, me, mf, mv)

	assert.Nil(t, res)
	assert500(t, err)
}

func TestHandlerConfig_GetConfig(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	me.On("NewVCHFromID", fakeData.ID).Return(fakeVM, nil)
	mv.On("SetDataFromVM", fakeOp, fakeVM, fakeData).Return(nil)
	me.On("GetNoSecretVCHConfig", fakeVM).Return(fakeConfig, nil)

	res, err := hc.GetVCHConfig(fakeOp, fakeData)

	checkMocks(t, me, mf, mv)

	assert.Equal(t, res, fakeConfig)
	assert.Nil(t, err)
}

func TestHandlerConfig_GetConfig_LookupError(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	me.On("NewVCHFromID", fakeData.ID).Return(nil, errFake)

	res, err := hc.GetVCHConfig(fakeOp, fakeData)

	checkMocks(t, me, mf, mv)

	assert.Nil(t, res)
	assert404(t, err)
}

func TestHandlerConfig_GetConfig_LoadError(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	me.On("NewVCHFromID", fakeData.ID).Return(fakeVM, nil)
	mv.On("SetDataFromVM", fakeOp, fakeVM, fakeData).Return(errFake)

	res, err := hc.GetVCHConfig(fakeOp, fakeData)

	checkMocks(t, me, mf, mv)

	assert.Nil(t, res)
	assert500(t, err)
}

func TestHandlerConfig_GetConfig_ConfigLoadError(t *testing.T) {
	hc, me, mf, mv := makeMocks()

	me.On("NewVCHFromID", fakeData.ID).Return(fakeVM, nil)
	mv.On("SetDataFromVM", fakeOp, fakeVM, fakeData).Return(nil)
	me.On("GetNoSecretVCHConfig", fakeVM).Return(nil, errFake)

	res, err := hc.GetVCHConfig(fakeOp, fakeData)

	checkMocks(t, me, mf, mv)

	assert.Nil(t, res)
	assert500(t, err)
}

func TestHandlerConfig_GetAddresses_NoTLS(t *testing.T) {
	fakeVCH := &config.VirtualContainerHostConfigSpec{
		ExecutorConfig: lib_executor.ExecutorConfig{
			Networks: map[string]*lib_executor.NetworkEndpoint{
				"client": {
					Assigned: net.IPNet{
						IP: net.ParseIP(fakeIP),
					},
				},
			},
		},
	}

	hc, me, mf, mv := makeMocks()

	dockerRes, adminRes, err := hc.GetAddresses(fakeVCH)

	checkMocks(t, me, mf, mv)

	assert.NotNil(t, dockerRes)
	assert.Contains(t, dockerRes, fakeIP)
	assert.Contains(t, dockerRes, fmt.Sprintf("%d", opts.DefaultHTTPPort))
	assert.NotNil(t, adminRes)
	assert.Contains(t, adminRes, fakeIP)
	assert.Contains(t, adminRes, fmt.Sprintf("%d", constants.VchAdminPortalPort))
	assert.Nil(t, err)
}

func TestHandlerConfig_GetAddresses_TLS(t *testing.T) {
	fakeVCH := &config.VirtualContainerHostConfigSpec{
		ExecutorConfig: lib_executor.ExecutorConfig{
			Networks: map[string]*lib_executor.NetworkEndpoint{
				"client": {
					Assigned: net.IPNet{
						IP: net.ParseIP(fakeIP),
					},
				},
			},
		},
		Certificate: config.Certificate{
			HostCertificate: &config.RawCertificate{
				Key:  fakeKey.Bytes,
				Cert: []byte(fakeCertificate),
			},
		},
	}

	hc, me, mf, mv := makeMocks()

	me.On("GetTLSFriendlyHostIP", mock.Anything, mock.Anything, mock.Anything).Return(fakeHost)

	dockerRes, adminRes, err := hc.GetAddresses(fakeVCH)

	checkMocks(t, me, mf, mv)

	assert.NotNil(t, dockerRes)
	assert.Contains(t, dockerRes, fakeHost)
	assert.Contains(t, dockerRes, fmt.Sprintf("%d", opts.DefaultTLSHTTPPort))
	assert.NotNil(t, adminRes)
	assert.Contains(t, adminRes, fakeHost)
	assert.Contains(t, adminRes, fmt.Sprintf("%d", constants.VchAdminPortalPort))
	assert.Nil(t, err)
}

func TestHandlerConfig_GetAddresses_Unspecified(t *testing.T) {
	fakeVCH := &config.VirtualContainerHostConfigSpec{
		ExecutorConfig: lib_executor.ExecutorConfig{
			Networks: map[string]*lib_executor.NetworkEndpoint{
				"client": {
					Assigned: net.IPNet{
						IP: net.IPv4zero,
					},
				},
			},
		},
	}

	hc, me, mf, mv := makeMocks()

	dockerRes, adminRes, err := hc.GetAddresses(fakeVCH)

	checkMocks(t, me, mf, mv)

	assert.Empty(t, dockerRes)
	assert.Empty(t, adminRes)
	assert.NotNil(t, err)
}

func TestHandlerConfig_GetAddresses_NoIp(t *testing.T) {
	fakeVCH := &config.VirtualContainerHostConfigSpec{
		ExecutorConfig: lib_executor.ExecutorConfig{
			Networks: map[string]*lib_executor.NetworkEndpoint{},
		},
	}

	hc, me, mf, mv := makeMocks()

	dockerRes, adminRes, err := hc.GetAddresses(fakeVCH)

	checkMocks(t, me, mf, mv)

	assert.Empty(t, dockerRes)
	assert.Empty(t, adminRes)
	assert.NotNil(t, err)
}
