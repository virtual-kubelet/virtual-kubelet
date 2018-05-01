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

package tether

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"runtime"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/crypto/ssh"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/etcconf"
	"github.com/vmware/vic/lib/system"
	"github.com/vmware/vic/pkg/dio"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

var Tthr Tether

type Mocker struct {
	Base BaseOperations

	// allow tests to tell when the tether has finished setup
	Started chan bool
	// allow tests to tell when the tether has finished
	Cleaned chan bool
	// allow tests to wait for a reload to complete
	Reloaded chan bool

	// debug output gets logged here
	LogWriter io.Writer

	// session output gets logged here
	SessionLogBuffer bytes.Buffer

	// the hostname of the system
	Hostname string
	Aliases  []string
	// the maximum slot number returned from this mocker
	maxSlot int
	// the interfaces in the system indexed by name
	Interfaces map[string]netlink.Link
	// filesystem mounts, indexed by disk label
	Mounts map[string]string

	WindowCol uint32
	WindowRow uint32
	Signal    ssh.Signal

	FirewallRules []string
}

// Start implements the extension method
func (t *Mocker) Start() error {
	return nil
}

// Stop implements the extension method
func (t *Mocker) Stop() error {
	close(t.Cleaned)

	defer func() {
		// tolerate closing started again
		recover()
	}()
	close(t.Started)
	close(t.Reloaded)
	return nil
}

// Reload implements the extension method
func (t *Mocker) Reload(config *ExecutorConfig) error {
	// the tether has definitely finished it's startup by the time we hit this
	defer func() {
		// if we trigger another Reload we don't want to panic
		recover()
	}()

	t.Reloaded <- true
	close(t.Started)
	return nil
}

func (t *Mocker) Cleanup() error {
	return nil
}

func (t *Mocker) Setup(c Config) error {
	return t.Base.Setup(c)
}

func (t *Mocker) Log() (io.Writer, error) {
	if t.LogWriter != nil {
		return t.LogWriter, nil
	}

	return os.Stdout, nil
}

func (t *Mocker) SetupFirewall(ctx context.Context, conf *ExecutorConfig) error {
	return nil
}

func (t *Mocker) SessionLog(session *SessionConfig) (dio.DynamicMultiWriter, dio.DynamicMultiWriter, error) {
	return dio.MultiWriter(&t.SessionLogBuffer), dio.MultiWriter(&t.SessionLogBuffer), nil
}

func (t *Mocker) HandleSessionExit(config *ExecutorConfig, session *SessionConfig) func() {
	// check for executor behaviour
	return func() {
		if session.ID == config.ID {
			Tthr.Stop()
		}
	}
}

func (t *Mocker) ProcessEnv(env []string) []string {
	return t.Base.ProcessEnv(env)
}

// SetHostname sets both the kernel hostname and /etc/hostname to the specified string
func (t *Mocker) SetHostname(hostname string, aliases ...string) error {
	defer trace.End(trace.Begin("mocking hostname to " + hostname))

	// TODO: we could mock at a much finer granularity, only extracting the syscall
	// that would exercise the file modification paths, however it's much less generalizable
	t.Hostname = hostname
	t.Aliases = aliases
	return nil
}

// Apply takes the network endpoint configuration and applies it to the system
func (t *Mocker) Apply(endpoint *NetworkEndpoint) error {
	return ApplyEndpoint(t, &t.Base, endpoint)
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
func (t *Mocker) LaunchUtility(fn UtilityFn) (<-chan int, error) {
	return launchUtility(&t.Base, fn)
}

func (t *Mocker) HandleUtilityExit(pid, exitCode int) bool {
	return handleUtilityExit(&t.Base, pid, exitCode)
}

// TestMain simply so we have control of debugging level and somewhere to call package wide test setup
func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)
	trace.Logger = log.StandardLogger()

	// restore what we inherited.
	defer func(hosts etcconf.Hosts, resolv etcconf.ResolvConf) {
		Sys.Hosts = hosts
		Sys.ResolvConf = resolv
	}(Sys.Hosts, Sys.ResolvConf)

	// replace the Sys variable with a mock
	r, err := ioutil.TempDir("", "tether-root")
	if err != nil {
		os.Exit(1)
		log.Fatalf("Failed to create tmpdir for test root filesytem: %s", err)
	}

	Sys = system.NewWithRoot(r)
	Sys.Syscall = &MockSyscall{}

	BindSys = system.NewWithRoot(path.Join(Sys.Root, "/.tether"))
	BindSys.Syscall = &MockSyscall{}

	// ensure that the bindsys root exists
	// #nosec
	os.MkdirAll(BindSys.Root, 0744)

	log.Debugf("Configured sys and bindsys to root in %s and %s", Sys.Root, BindSys.Root)

	retCode := m.Run()

	// call with result of m.Run()
	os.Exit(retCode)
}

func StartTether(t *testing.T, cfg *executor.ExecutorConfig, mocker *Mocker) (Tether, extraconfig.DataSource, extraconfig.DataSink) {
	store := extraconfig.New()
	sink := store.Put
	src := store.Get
	extraconfig.Encode(sink, cfg)
	log.Debugf("Test configuration: %#v", sink)

	Tthr = New(src, sink, mocker)
	Tthr.Register("mocker", mocker)

	// run the tether to service the attach
	go func() {
		erR := Tthr.Start()
		if erR != nil {
			t.Error(erR)
		}
	}()

	return Tthr, src, sink
}

func RunTether(t *testing.T, cfg *executor.ExecutorConfig, mocker *Mocker) (Tether, extraconfig.DataSource, error) {
	store := extraconfig.New()
	sink := store.Put
	src := store.Get
	extraconfig.Encode(sink, cfg)
	log.Debugf("Test configuration: %#v", sink)

	Tthr = New(src, sink, mocker)
	Tthr.Register("Mocker", mocker)

	// run the tether to service the attach
	erR := Tthr.Start()

	return Tthr, src, erR
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

func testSetup(t *testing.T) (string, *Mocker) {
	pc, _, _, _ := runtime.Caller(1)
	name := runtime.FuncForPC(pc).Name()

	log.Infof("Started test setup for %s", name)

	// use the mock ops - fresh one each time as tests might apply different mocked calls
	mocker := &Mocker{
		Started:    make(chan bool, 0),
		Cleaned:    make(chan bool, 0),
		Reloaded:   make(chan bool, 100),
		Interfaces: make(map[string]netlink.Link, 0),
	}

	return name, mocker
}

func testTeardown(t *testing.T, mocker *Mocker) {
	// cleanup
	// os.RemoveAll(pathPrefix)
	log.SetOutput(os.Stdout)

	<-mocker.Cleaned

	pc, _, _, _ := runtime.Caller(1)
	name := runtime.FuncForPC(pc).Name()

	log.Infof("Finished test teardown for %s", name)
}
