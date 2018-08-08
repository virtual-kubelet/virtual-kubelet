// Copyright 2016 VMware, Inc. All Rights Reserved.
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

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"runtime"
	"testing"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/system"
	"github.com/vmware/vic/lib/tether"
	"github.com/vmware/vic/pkg/dio"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

var Mocked Mocker

type Mocker struct {
	Base tether.BaseOperations

	// allow tests to tell when the tether has finished setup
	Started chan bool
	// allow tests to tell when the tether has finished
	Cleaned chan bool
	// Session exit
	SessionExit chan bool

	// debug output gets logged here
	LogBuffer bytes.Buffer

	// session output gets logged here
	SessionLogBuffer bytes.Buffer

	// the hostname of the system
	Hostname string
	// the ip configuration for name index networks
	IPs map[string]net.IP
	// filesystem mounts, indexed by disk label
	Mounts map[string]string

	WindowCol uint32
	WindowRow uint32
	Signal    ssh.Signal
}

// Start implements the extension method
func (t *Mocker) Start(system tether.System) error {
	return nil
}

// Stop implements the extension method
func (t *Mocker) Stop() error {
	close(t.Cleaned)
	return nil
}

// Reload implements the extension method
func (t *Mocker) Reload(config *tether.ExecutorConfig) error {
	// the tether has definitely finished it's startup by the time we hit this
	defer func() {
		// deal with repeated reloads
		recover()
	}()

	close(t.Started)

	return nil
}

func (t *Mocker) Setup(tether.Config) error {
	return nil
}

func (t *Mocker) Cleanup() error {
	return nil
}

func (t *Mocker) Log() (io.Writer, error) {
	return &t.LogBuffer, nil
}

func (t *Mocker) SessionLog(session *tether.SessionConfig) (dio.DynamicMultiWriter, dio.DynamicMultiWriter, error) {
	return dio.MultiWriter(&t.SessionLogBuffer), dio.MultiWriter(&t.SessionLogBuffer), nil
}

func (t *Mocker) HandleSessionExit(config *tether.ExecutorConfig, session *tether.SessionConfig) func() {
	return func() {
		t.SessionExit <- true
	}
}

func (t *Mocker) ProcessEnv(session *tether.SessionConfig) []string {
	return t.Base.ProcessEnv(session)
}

// SetHostname sets both the kernel hostname and /etc/hostname to the specified string
func (t *Mocker) SetHostname(hostname string, aliases ...string) error {
	defer trace.End(trace.Begin("mocking hostname to " + hostname))

	// TODO: we could mock at a much finer granularity, only extracting the syscall
	// that would exercise the file modification paths, however it's much less generalizable
	t.Hostname = hostname
	return nil
}

func (t *Mocker) SetupFirewall(cxt context.Context, conf *tether.ExecutorConfig) error {
	return nil
}

// Apply takes the network endpoint configuration and applies it to the system
func (t *Mocker) Apply(endpoint *tether.NetworkEndpoint) error {
	defer trace.End(trace.Begin("mocking endpoint configuration for " + endpoint.Network.Name))
	t.IPs[endpoint.Network.Name] = endpoint.Assigned.IP

	return nil
}

// MountLabel performs a mount with the source treated as a disk label
// This assumes that /dev/disk/by-label is being populated, probably by udev
func (t *Mocker) MountLabel(ctx context.Context, label, target string) error {
	defer trace.End(trace.Begin(fmt.Sprintf("mocking mounting %s on %s", label, target)))

	if t.Mounts == nil {
		t.Mounts = make(map[string]string)
	}

	t.Mounts[label] = target
	return nil
}

// MountTarget performs a mount with the source treated as an nfs target
func (t *Mocker) MountTarget(ctx context.Context, source url.URL, target string, mountOptions string) error {
	defer trace.End(trace.Begin(fmt.Sprintf("mocking mounting %s on %s", source.String(), target)))

	if t.Mounts == nil {
		t.Mounts = make(map[string]string)
	}

	t.Mounts[source.String()] = target
	return nil
}

// CopyExistingContent copies the underlying files shadowed by a mount on a directory
// to the volume mounted on the directory
func (t *Mocker) CopyExistingContent(source string) error {
	defer trace.End(trace.Begin(fmt.Sprintf("mocking copyExistingContent from %s", source)))
	return nil
}

// Fork triggers vmfork and handles the necessary pre/post OS level operations
func (t *Mocker) Fork() error {
	defer trace.End(trace.Begin("mocking fork"))
	return errors.New("Fork test not implemented")
}

// LaunchUtility uses the underlying implementation for launching and tracking utility processes
func (t *Mocker) LaunchUtility(fn tether.UtilityFn) (<-chan int, error) {
	return t.Base.LaunchUtility(fn)
}

func (t *Mocker) HandleUtilityExit(pid, exitCode int) bool {
	return t.Base.HandleUtilityExit(pid, exitCode)
}

// TestMain simply so we have control of debugging level and somewhere to call package wide test setup
func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)
	trace.Logger = log.StandardLogger()

	// replace the Sys variable with a mock
	tether.Sys = system.System{
		Hosts:      &tether.MockHosts{},
		ResolvConf: &tether.MockResolvConf{},
		Syscall:    &tether.MockSyscall{},
		Root:       os.TempDir(),
	}

	retCode := m.Run()

	// call with result of m.Run()
	os.Exit(retCode)
}

func StartTether(t *testing.T, cfg *executor.ExecutorConfig) (tether.Tether, extraconfig.DataSource) {
	store := extraconfig.New()
	sink := store.Put
	src := store.Get
	extraconfig.Encode(sink, cfg)
	log.Debugf("Test configuration: %#v", sink)

	tthr = tether.New(src, sink, &Mocked)
	tthr.Register("mocker", &Mocked)

	// run the tether to service the attach
	go func() {
		err := tthr.Start()
		if err != nil {
			t.Error(err)
		}
	}()

	return tthr, src
}

func RunTether(t *testing.T, cfg *executor.ExecutorConfig) (tether.Tether, extraconfig.DataSource, error) {
	store := extraconfig.New()
	sink := store.Put
	src := store.Get
	extraconfig.Encode(sink, cfg)
	log.Debugf("Test configuration: %#v", sink)

	tthr = tether.New(src, sink, &Mocked)
	tthr.Register("Mocker", &Mocked)

	// run the tether to service the attach
	erR := tthr.Start()

	return tthr, src, erR
}

func OptionValueArrayToString(options []types.BaseOptionValue) string {
	// create the key/value store from the extraconfig slice for lookups
	kv := make(map[string]string)
	for i := range options {
		k := options[i].GetOptionValue().Key
		v := options[i].GetOptionValue().Value.(string)
		kv[k] = v
	}

	return fmt.Sprintf("%#v", kv)
}

func testSetup(t *testing.T) {
	pc, _, _, _ := runtime.Caller(1)
	name := runtime.FuncForPC(pc).Name()

	log.Infof("Started test setup for %s", name)

	// use the mock ops - fresh one each time as tests might apply different mocked calls
	Mocked = Mocker{
		Started:     make(chan bool),
		Cleaned:     make(chan bool),
		SessionExit: make(chan bool),
	}
}

func testTeardown(t *testing.T) {
	// cleanup
	<-Mocked.Cleaned

	os.RemoveAll(pathPrefix)
	log.SetOutput(os.Stdout)

	pc, _, _, _ := runtime.Caller(1)
	name := runtime.FuncForPC(pc).Name()

	log.Infof("Finished test teardown for %s", name)
}
