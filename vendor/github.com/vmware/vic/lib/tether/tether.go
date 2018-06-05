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

package tether

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof" // allow enabling pprof in containerVM
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime/debug"
	"sort"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/system"
	"github.com/vmware/vic/lib/tether/msgs"
	"github.com/vmware/vic/lib/tether/shared"
	"github.com/vmware/vic/pkg/dio"
	"github.com/vmware/vic/pkg/log/syslog"
	"github.com/vmware/vic/pkg/serial"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"

	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	// MaxDeathRecords The maximum number of records to keep for restarting processes
	MaxDeathRecords = 5

	// the length of a truncated ID for use as hostname
	shortLen = 12
)

// Sys is used to configure where the target system files are
var (
	// Used to access the acutal system paths and files
	Sys = shared.Sys
	// Used to access and manipulate the tether modified bind sources
	// that are mounted over the system ones.
	BindSys = system.NewWithRoot("/.tether")
	once    sync.Once
)

const (
	useRunc  = true
	runcPath = "/sbin/runc"
)

type tether struct {
	// the implementation to use for tailored operations
	ops Operations

	// the reload channel is used to block reloading of the config
	reload chan struct{}

	// config holds the main configuration for the executor
	config *ExecutorConfig

	// a set of extensions that get to operate on the config
	extensions map[string]Extension

	src  extraconfig.DataSource
	sink extraconfig.DataSink

	// Cancelable context and its cancel func.
	ctx    context.Context
	cancel context.CancelFunc

	incoming chan os.Signal

	// syslog writer shared by all sessions
	writer syslog.Writer

	// used for running vm initialization logic once in the reload loop
	initialize sync.Once
}

func New(src extraconfig.DataSource, sink extraconfig.DataSink, ops Operations) Tether {
	ctx, cancel := context.WithCancel(context.Background())
	return &tether{
		ops:    ops,
		reload: make(chan struct{}, 1),
		config: &ExecutorConfig{
			pids: make(map[int]*SessionConfig),
		},
		extensions: make(map[string]Extension),
		src:        src,
		sink:       sink,
		ctx:        ctx,
		cancel:     cancel,
		incoming:   make(chan os.Signal, 32),
	}
}

// removeChildPid is a synchronized accessor for the pid map the deletes the entry and returns the value
func (t *tether) removeChildPid(pid int) (*SessionConfig, bool) {
	t.config.pidMutex.Lock()
	defer t.config.pidMutex.Unlock()

	session, ok := t.config.pids[pid]
	delete(t.config.pids, pid)
	return session, ok
}

// lenChildPid returns the number of entries
func (t *tether) lenChildPid() int {
	t.config.pidMutex.Lock()
	defer t.config.pidMutex.Unlock()

	return len(t.config.pids)
}

func (t *tether) setup() error {
	defer trace.End(trace.Begin("main tether setup"))

	// set up tether logging destination
	out, err := t.ops.Log()
	if err != nil {
		log.Errorf("failed to open tether log: %s", err)
		return err
	}
	if out != nil {
		log.SetOutput(out)
	}

	t.reload = make(chan struct{}, 1)
	t.config = &ExecutorConfig{
		pids: make(map[int]*SessionConfig),
	}

	if err := t.childReaper(); err != nil {
		log.Errorf("Failed to start reaper %s", err)
		return err
	}

	if err := t.ops.Setup(t); err != nil {
		log.Errorf("Failed tether setup: %s", err)
		return err
	}

	for name, ext := range t.extensions {
		log.Infof("Starting extension %s", name)
		err := ext.Start()
		if err != nil {
			log.Errorf("Failed to start extension %s: %s", name, err)
			return err
		}
	}

	pidDir := shared.PIDFileDir()

	// #nosec: Expect directory permissions to be 0700 or less
	if err = os.MkdirAll(pidDir, 0755); err != nil {
		log.Errorf("could not create pid file directory %s: %s", pidDir, err)
	}

	// Create PID file for tether
	tname := path.Base(os.Args[0])
	err = ioutil.WriteFile(fmt.Sprintf("%s.pid", path.Join(pidDir, tname)),
		[]byte(fmt.Sprintf("%d", os.Getpid())),
		0644)
	if err != nil {
		log.Errorf("Unable to open PID file for %s : %s", os.Args[0], err)
	}

	// seed the incoming channel once to trigger child reaper. This is required to collect the zombies created by switch-root
	t.triggerReaper()

	return nil
}

func (t *tether) cleanup() {
	defer trace.End(trace.Begin("main tether cleanup"))

	// stop child reaping
	t.stopReaper()

	// stop the extensions first as they may use the config
	for name, ext := range t.extensions {
		log.Infof("Stopping extension %s", name)
		err := ext.Stop()
		if err != nil {
			log.Warnf("Failed to cleanly stop extension %s", name)
		}
	}

	// return logging to standard location
	log.SetOutput(os.Stdout)

	// perform basic cleanup
	t.ops.Cleanup()
}

func (t *tether) setLogLevel() {
	// TODO: move all of this into an extension.Pre() block when we move to that model
	// adjust the logging level appropriately
	log.SetLevel(log.InfoLevel)
	// TODO: do not echo application output to console without debug enabled
	serial.DisableTracing()

	if t.config.DebugLevel > 0 {
		log.SetLevel(log.DebugLevel)

		logConfig(t.config)
	}

	if t.config.DebugLevel > 1 {
		serial.EnableTracing()

		log.Info("Launching pprof server on port 6060")
		fn := func() {
			go http.ListenAndServe("0.0.0.0:6060", nil)
		}

		once.Do(fn)
	}

	if t.config.DebugLevel > 3 {
		// extraconfig is very, very verbose
		extraconfig.SetLogLevel(log.DebugLevel)
	}
}

func (t *tether) setHostname() error {
	short := t.config.ID
	if len(short) > shortLen {
		short = short[:shortLen]
	}

	full := t.config.Hostname
	if t.config.Hostname != "" && t.config.Domainname != "" {
		full = fmt.Sprintf("%s.%s", t.config.Hostname, t.config.Domainname)
	}

	hostname := short
	if full != "" {
		hostname = full
	}

	if err := t.ops.SetHostname(hostname, t.config.Name); err != nil {
		// we don't attempt to recover from this - it's a fundamental misconfiguration
		// so just exit
		return fmt.Errorf("failed to set hostname: %s", err)
	}
	return nil
}

func (t *tether) setNetworks() error {

	// internal networks must be applied first
	for _, network := range t.config.Networks {
		if !network.Internal {
			continue
		}
		if err := t.ops.Apply(network); err != nil {
			return fmt.Errorf("failed to apply network endpoint config: %s", err)
		}
	}
	for _, network := range t.config.Networks {
		if network.Internal {
			continue
		}
		if err := t.ops.Apply(network); err != nil {
			return fmt.Errorf("failed to apply network endpoint config: %s", err)
		}
	}

	return nil
}

func (t *tether) setMounts() error {
	// provides a lookup from path to volume reference.
	pathIndex := make(map[string]string, 0)
	mounts := make([]string, 0, len(t.config.Mounts))
	for k, v := range t.config.Mounts {
		log.Infof("** mount = %#v", v)
		mounts = append(mounts, v.Path)
		pathIndex[v.Path] = k
	}
	// Order the mount paths so that we are doing them in shortest order first.
	sort.Strings(mounts)

	for _, v := range mounts {
		targetRef := pathIndex[v]
		mountTarget := t.config.Mounts[targetRef]
		switch mountTarget.Source.Scheme {
		case "label":
			// this could block indefinitely while waiting for a volume to present
			// return error if mount volume failed, to fail container start
			if err := t.ops.MountLabel(context.Background(), mountTarget.Source.Path, mountTarget.Path); err != nil {
				return err
			}
		case "nfs":
			// return error if mount nfs volume failed, to fail container start, so user knows there is something wrong
			if err := t.ops.MountTarget(context.Background(), mountTarget.Source, mountTarget.Path, mountTarget.Mode); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported volume mount type for %s: %s", targetRef, mountTarget.Source.Scheme)
		}
	}

	// FIXME: populateVolumes() does not handle the nested volume case properly.
	return t.populateVolumes()
}

func (t *tether) populateVolumes() error {
	defer trace.End(trace.Begin(fmt.Sprintf("populateVolumes")))
	// skip if no mounts present
	if len(t.config.Mounts) == 0 {
		return nil
	}

	for _, mnt := range t.config.Mounts {
		if mnt.Path == "" {
			continue
		}
		if mnt.CopyMode == executor.CopyNew {
			err := t.ops.CopyExistingContent(mnt.Path)
			if err != nil {
				log.Errorf("error copyExistingContent for mount %s: %+v", mnt.Path, err)
				return err
			}
		}
	}

	return nil
}

func (t *tether) initializeSessions() error {

	maps := map[string]map[string]*SessionConfig{
		"Sessions": t.config.Sessions,
		"Execs":    t.config.Execs,
	}

	// we need to iterate over both sessions and execs
	for name, m := range maps {
		// Iterate over the Sessions and initialize them if needed
		for id, session := range m {
			log.Debugf("Initializing session %s", id)

			session.Lock()
			if session.wait != nil {
				log.Warnf("Session %s already initialized", id)
			} else {
				if session.RunBlock {
					log.Infof("Session %s wants attach capabilities. Creating its channel", id)
					session.ClearToLaunch = make(chan struct{})
				}

				// this will need altering if tether should be capable of being restarted itself
				session.Started = ""
				session.StopTime = 0

				session.wait = &sync.WaitGroup{}
				session.extraconfigKey = name
				err := t.loggingLocked(session)
				if err != nil {
					log.Errorf("initializing logging for session failed with %s", err)
					session.Unlock()
					return err
				}
			}
			session.Unlock()
		}
	}
	return nil
}

func (t *tether) reloadExtensions() error {
	// reload the extensions
	for name, ext := range t.extensions {
		log.Debugf("Passing config to %s", name)
		err := ext.Reload(t.config)
		if err != nil {
			return fmt.Errorf("Failed to cleanly reload config for extension %s: %s", name, err)
		}
	}
	return nil
}

func (t *tether) processSessions() error {
	type results struct {
		id    string
		path  string
		err   error
		fatal bool
	}

	// so that we can launch multiple sessions in parallel
	var wg sync.WaitGroup
	// to collect the errors back from them
	resultsCh := make(chan results, len(t.config.Sessions)+len(t.config.Execs))

	maps := []struct {
		sessions map[string]*SessionConfig
		fatal    bool
	}{
		{t.config.Sessions, true},
		{t.config.Execs, false},
	}

	// we need to iterate over both sessions and execs
	for i := range maps {
		m := maps[i]

		// process the sessions and launch if needed
		for id, session := range m.sessions {
			func() {
				session.Lock()
				defer session.Unlock()

				log.Debugf("Processing config for session: %s", id)
				var proc = session.Cmd.Process

				// check if session is alive and well
				if proc != nil && proc.Signal(syscall.Signal(0)) == nil {
					log.Debugf("Process for session %s is running (pid: %d)", id, proc.Pid)
					if !session.Active {
						// stop process - for now this doesn't do any staged levels of aggression
						log.Infof("Running session %s has been deactivated (pid: %d)", id, proc.Pid)

						killHelper(session)
					}

					return
				}

				// if we're not activating this session and it's not running, then skip
				if !session.Active {
					log.Debugf("Skipping inactive session: %s", id)
					return
				}

				priorLaunch := proc != nil || session.Started != ""
				if priorLaunch && !session.Restart {
					log.Debugf("Skipping non-restartable exited or failed session: %s", id)
					return
				}

				if !priorLaunch {
					log.Infof("Launching process for session %s", id)
					log.Debugf("Launch failures are fatal: %t", m.fatal)
				} else {
					session.Diagnostics.ResurrectionCount++

					// FIXME: we cannot have this embedded knowledge of the extraconfig encoding pattern, but not
					// currently sure how to expose it neatly via a utility function
					extraconfig.EncodeWithPrefix(t.sink, session, extraconfig.CalculateKeys(t.config, fmt.Sprintf("%s.%s", session.extraconfigKey, id), "")[0])
					log.Warnf("Re-launching process for session %s (count: %d)", id, session.Diagnostics.ResurrectionCount)
					session.Cmd = *restartableCmd(&session.Cmd)
				}

				wg.Add(1)
				go func(session *SessionConfig) {
					defer wg.Done()
					resultsCh <- results{
						id:    session.ID,
						path:  session.Cmd.Path,
						err:   t.launch(session),
						fatal: m.fatal,
					}
				}(session)
			}()
		}
	}

	wg.Wait()
	// close the channel
	close(resultsCh)

	// iterate over the results
	for result := range resultsCh {
		if result.err != nil {
			detail := fmt.Errorf("failed to launch %s for %s: %s", result.path, result.id, result.err)
			if result.fatal {
				log.Error(detail)
				return detail
			}

			log.Warn(detail)
			return nil
		}
	}
	return nil
}

type TetherKey struct{}

func (t *tether) Start() error {
	defer trace.End(trace.Begin("main tether loop"))

	defer func() {
		e := recover()
		if e != nil {
			log.Errorf("Panic in main tether loop: %s: %s", e, debug.Stack())
			// continue panicing now it's logged
			panic(e)
		}
	}()

	// do the initial setup and start the extensions
	if err := t.setup(); err != nil {
		log.Errorf("Failed to run setup: %s", err)
		return err
	}
	defer t.cleanup()

	// initial entry, so seed this
	t.reload <- struct{}{}
	for range t.reload {
		var err error

		select {
		case <-t.ctx.Done():
			log.Warnf("Someone called shutdown, returning from start")
			return nil
		default:
		}
		log.Info("Loading main configuration")

		// load the config - this modifies the structure values in place
		t.config.Lock()
		extraconfig.Decode(t.src, t.config)
		t.config.Unlock()

		t.setLogLevel()

		// TODO: this ensures that we run vm related setup code once
		// This is temporary as none of those functions are idempotent at this point
		// https://github.com/vmware/vic/issues/5833
		t.initialize.Do(func() {
			if err = t.setHostname(); err != nil {
				return
			}

			// process the networks then publish any dynamic data
			if err = t.setNetworks(); err != nil {
				return
			}
			extraconfig.Encode(t.sink, t.config)

			// setup the firewall
			if err = retryOnError(func() error { return t.ops.SetupFirewall(t.ctx, t.config) }, 5); err != nil {
				err = fmt.Errorf("Couldn't set up container-network firewall: %v", err)
				return
			}

			//process the filesystem mounts - this is performed after networks to allow for network mounts
			if err = t.setMounts(); err != nil {
				return
			}
		})

		if err != nil {
			log.Error(err)
			return err
		}

		if err = t.initializeSessions(); err != nil {
			log.Error(err)
			return err
		}

		// Danger, Will Robinson! There is a strict ordering here.
		// We need to start attach server first so that it can unblock the session
		if err = t.reloadExtensions(); err != nil {
			log.Error(err)
			return err
		}

		if err = t.processSessions(); err != nil {
			log.Error(err)
			return err
		}
	}

	log.Info("Finished processing sessions")

	return nil
}

func (t *tether) Stop() error {
	defer trace.End(trace.Begin(""))

	// cancel the context to signal waiters
	t.cancel()

	// TODO: kill all the children
	close(t.reload)

	return nil
}

func (t *tether) Reload() {
	defer trace.End(trace.Begin(""))

	select {
	case <-t.ctx.Done():
		log.Warnf("Someone called shutdown, dropping the reload request")
		return
	default:
		t.reload <- struct{}{}
	}
}

func (t *tether) Register(name string, extension Extension) {
	log.Infof("Registering tether extension " + name)

	t.extensions[name] = extension
}

func retryOnError(cmd func() error, maximumAttempts int) error {
	for i := 0; i < maximumAttempts-1; i++ {
		if err := cmd(); err != nil {
			log.Warningf("Failed with error \"%v\". Retrying (Attempt %v).", err, i+1)
		} else {
			return nil
		}
	}
	return cmd()
}

// cleanupSession performs some common cleanup work between handling a session exit and
// handling a failure to launch
// caller needs to hold session Lock
func (t *tether) cleanupSession(session *SessionConfig) {
	// close down the outputs
	log.Debugf("Calling close on writers")
	if session.Outwriter != nil {
		if err := session.Outwriter.Close(); err != nil {
			log.Warnf("Close for Outwriter returned %s", err)
		}
	}

	// this is a little ugly, however ssh channel.Close will get invoked by these calls,
	// whereas CloseWrite will be invoked by the OutWriter.Close so that goes first.
	if session.Errwriter != nil {
		if err := session.Errwriter.Close(); err != nil {
			log.Warnf("Close for Errwriter returned %s", err)
		}
	}

	// if we're calling this we don't care about truncation of pending input, so this is
	// called last
	if session.Reader != nil {
		log.Debugf("Calling close on reader")
		if err := session.Reader.Close(); err != nil {
			log.Warnf("Close for Reader returned %s", err)
		}
	}

	// close the signaling channel (it is nil for detached sessions) and set it to nil (for restart)
	if session.ClearToLaunch != nil {
		log.Debugf("Calling close chan: %s", session.ID)
		close(session.ClearToLaunch)
		session.ClearToLaunch = nil
		// reset Runblock to unblock process start next time
		session.RunBlock = false
	}
}

// handleSessionExit processes the result from the session command, records it in persistent
// maner and determines if the Executor should exit
// caller needs to hold session Lock
func (t *tether) handleSessionExit(session *SessionConfig) {
	defer trace.End(trace.Begin("handling exit of session " + session.ID))

	log.Debugf("Waiting on session.wait")
	session.wait.Wait()
	log.Debugf("Wait on session.wait completed")

	log.Debugf("Calling wait on cmd")
	if err := session.Cmd.Wait(); err != nil {
		// we expect this to get an error because the child reaper will have gathered it
		log.Debugf("Wait returned %s", err)
	}

	t.cleanupSession(session)

	// Remove associated PID file
	cmdname := path.Base(session.Cmd.Path)

	_ = os.Remove(fmt.Sprintf("%s.pid", path.Join(shared.PIDFileDir(), cmdname)))

	// set the stop time
	session.StopTime = time.Now().UTC().Unix()

	// this returns an arbitrary closure for invocation after the session status update
	f := t.ops.HandleSessionExit(t.config, session)

	extraconfig.EncodeWithPrefix(t.sink, session, extraconfig.CalculateKeys(t.config, fmt.Sprintf("%s.%s", session.extraconfigKey, session.ID), "")[0])

	if f != nil {
		log.Debugf("Calling t.ops.HandleSessionExit")
		f()
	}
}

func (t *tether) loggingLocked(session *SessionConfig) error {
	stdout, stderr, err := t.ops.SessionLog(session)
	if err != nil {
		detail := fmt.Errorf("failed to get log writer for session: %s", err)
		session.Started = detail.Error()

		return detail
	}
	session.Outwriter = stdout
	session.Errwriter = stderr
	session.Reader = dio.MultiReader()

	if session.Diagnostics.SysLogConfig != nil {
		cfg := session.Diagnostics.SysLogConfig
		var w syslog.Writer
		if t.writer == nil {
			t.writer, err = syslog.Dial(cfg.Network, cfg.RAddr, syslog.Info|syslog.Daemon, fmt.Sprintf("%s", t.config.ID[:shortLen]))
			if err != nil {
				log.Warnf("could not connect to syslog server: %s", err)
			}
			w = t.writer
		} else {
			w = t.writer.WithTag(fmt.Sprintf("%s", t.config.ID[:shortLen]))
		}

		if w != nil {
			stdout.Add(w)
			stderr.Add(w.WithPriority(syslog.Err | syslog.Daemon))
		}
	}

	return nil
}

// launch will launch the command defined in the session.
// This will return an error if the session fails to launch
func (t *tether) launch(session *SessionConfig) error {
	defer trace.End(trace.Begin("launching session " + session.ID))

	session.Lock()
	defer session.Unlock()

	var err error
	defer func() {
		if session.Started != "true" {
			// if we didn't launch cleanly then clean up
			t.cleanupSession(session)
		}

		// encode the result whether success or error
		prefix := extraconfig.CalculateKeys(t.config, fmt.Sprintf("%s.%s", session.extraconfigKey, session.ID), "")[0]
		log.Debugf("Encoding result of launch for session %s under key: %s", session.ID, prefix)
		extraconfig.EncodeWithPrefix(t.sink, session, prefix)
	}()

	if len(session.User) > 0 || len(session.Group) > 0 {
		user, err := getUserSysProcAttr(session.User, session.Group)
		if err != nil {
			log.Errorf("user lookup failed %s:%s, %s", session.User, session.Group, err)
			session.Started = err.Error()
			return err
		}
		session.Cmd.SysProcAttr = user
	}

	rootfs := ""
	if session.ExecutionEnvironment != "" {
		// session is supposed to run in the specific filesystem environment
		// HACK: for now assume that the filesystem is mounted under /mnt/images/label, with label truncated to 15
		label := session.ExecutionEnvironment
		if len(label) > 15 {
			label = label[:15]
		}
		rootfs = fmt.Sprintf("/mnt/images/%s/rootfs", label)

		log.Infof("Configuring session to run in rootfs at %s", rootfs)
		log.Debugf("Updating %+v for chroot", session.Cmd.SysProcAttr)

		if useRunc {
			prepareOCI(session, rootfs)
		} else {
			session.Cmd.SysProcAttr = chrootSysProcAttr(session.Cmd.SysProcAttr, rootfs)
		}
	}

	if session.Diagnostics.ResurrectionCount > 0 {
		// override session logging only while it's restarted to avoid break exec #6004
		err = t.loggingLocked(session)
		if err != nil {
			log.Errorf("initializing logging for session failed with %s", err)
			return err
		}
	}

	session.Cmd.Env = t.ops.ProcessEnv(session.Cmd.Env)
	// Set Std{in|out|err} to nil, we will control pipes
	session.Cmd.Stdin = nil
	session.Cmd.Stdout = nil
	session.Cmd.Stderr = nil

	// Set StopTime to its default value
	session.StopTime = 0

	resolved, err := lookPath(session.Cmd.Path, session.Cmd.Env, session.Cmd.Dir, rootfs)
	if err != nil {
		log.Errorf("Path lookup failed for %s: %s", session.Cmd.Path, err)
		session.Started = err.Error()
		return err
	}
	log.Debugf("Resolved %s to %s", session.Cmd.Path, resolved)
	session.Cmd.Path = resolved

	// block until we have a connection
	if session.RunBlock && session.ClearToLaunch != nil {
		log.Infof("Waiting clear signal to launch %s", session.ID)
		select {
		case <-t.ctx.Done():
			log.Warnf("Waiting to launch %s canceled, bailing out", session.ID)
			return nil
		case <-session.ClearToLaunch:
			log.Infof("Received the clear signal to launch %s", session.ID)
		}
		// reset RunBlock to unblock process start next time
		session.RunBlock = false
	}

	pid := 0
	// Use the mutex to make creating a child and adding the child pid into the
	// childPidTable appear atomic to the reaper function. Use a anonymous function
	// so we can defer unlocking locally
	// logging is done after the function to keep the locked time as low as possible
	err = func() error {
		t.config.pidMutex.Lock()
		defer t.config.pidMutex.Unlock()

		if !session.Tty {
			err = establishNonPty(session)
		} else {
			err = establishPty(session)
		}
		if err != nil {
			return err
		}

		pid = session.Cmd.Process.Pid
		t.config.pids[pid] = session

		return nil
	}()

	if err != nil {
		detail := fmt.Sprintf("failed to start container process: %s", err)
		log.Error(detail)

		// Set the Started key to the undecorated error message
		session.Started = err.Error()

		return errors.New(detail)
	}

	// ensure that this is updated so that we're correct for out-of-band power operations
	// semantic should conform with port layer
	session.StartTime = time.Now().UTC().Unix()

	// Set the Started key to "true" - this indicates a successful launch
	session.Started = "true"

	// Write the PID to the associated PID file
	cmdname := path.Base(session.Cmd.Path)
	err = ioutil.WriteFile(fmt.Sprintf("%s.pid", path.Join(shared.PIDFileDir(), cmdname)),
		[]byte(fmt.Sprintf("%d", pid)),
		0644)
	if err != nil {
		log.Errorf("Unable to write PID file for %s: %s", cmdname, err)
	}
	log.Debugf("Launched command with pid %d", pid)

	return nil
}

// prepareOCI creates a config.json for the image.
func prepareOCI(session *SessionConfig, rootfs string) error {
	// Use runc to create a default spec
	/* #nosec */
	cmdRunc := exec.Command(runcPath, "spec")
	cmdRunc.Dir = path.Dir(rootfs)

	err := cmdRunc.Run()
	if err != nil {
		log.Errorf("Generating base OCI config file failed: %s", err.Error())
		return err
	}

	// Load the spec
	cfgFile := path.Join(cmdRunc.Dir, "config.json")
	configBytes, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.Errorf("Reading OCI config file failed: %s", err.Error())
		return err
	}

	var configSpec specs.Spec
	err = json.Unmarshal(configBytes, &configSpec)
	if err != nil {
		log.Errorf("Failed to unmarshal OCI config file: %s", err.Error())
		return err
	}

	// Modify the spec
	if configSpec.Root == nil {
		configSpec.Root = &specs.Root{}
	}
	//TODO:  Remove false default.  This is here only for debugging purposes.
	configSpec.Root.Readonly = false
	if configSpec.Process == nil {
		configSpec.Process = &specs.Process{}
	}

	configSpec.Process.Args = session.Cmd.Args

	// Save the spec out to file
	configBytes, err = json.Marshal(configSpec)
	if err != nil {
		log.Errorf("Failed to marshal OCI config file: %s", err.Error())
		return err
	}

	err = ioutil.WriteFile(cfgFile, configBytes, 0644)
	if err != nil {
		log.Errorf("Failed to write out updated OCI config file: %s", err.Error())
		return err
	}

	log.Infof("Updated OCI spec with process = %#v", *configSpec.Process)

	// Finally, update the session to call runc
	session.Cmd.Path = runcPath
	session.Cmd.Args = append(session.Cmd.Args, "run")
	session.Cmd.Args = append(session.Cmd.Args, "--no-pivot")
	// Use session.ID as container name
	session.Cmd.Args = append(session.Cmd.Args, session.ID)
	session.Cmd.Dir = path.Dir(rootfs)

	return nil
}

func logConfig(config *ExecutorConfig) {
	// just pretty print the json for now
	log.Info("Loaded executor config")

	// figure out the keys to filter
	keys := make(map[string]interface{})
	if config.DebugLevel < 2 {
		for _, f := range []string{
			"Sessions.*.Cmd.Args",
			"Sessions.*.Cmd.Args.*",
			"Sessions.*.Cmd.Env",
			"Sessions.*.Cmd.Env.*",
			"Key"} {
			for _, k := range extraconfig.CalculateKeys(config, f, "") {
				keys[k] = nil
			}
		}
	}

	sink := map[string]string{}
	extraconfig.Encode(
		func(k, v string) error {
			if _, ok := keys[k]; !ok {
				sink[k] = v
			}

			return nil
		},
		config,
	)

	for k, v := range sink {
		log.Debugf("%s: %s", k, v)
	}

}

func (t *tether) forkHandler() {
	defer trace.End(trace.Begin("start fork trigger handler"))

	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in forkHandler", r)
		}
	}()

	incoming := make(chan os.Signal, 1)
	signal.Notify(incoming, syscall.SIGABRT)

	log.Info("SIGABRT handling initialized for fork support")
	for range incoming {
		// validate that this is a fork trigger and not just a random signal from
		// container processes
		log.Info("Received SIGABRT - preparing to transition to fork parent")

		// TODO: record fork trigger in Config and persist

		// TODO: do we need to rebind session executions stdio to /dev/null or to files?
		err := t.ops.Fork()
		if err != nil {
			log.Errorf("vmfork failed:%s\n", err)
			// TODO: how do we handle fork failure behaviour at a container level?
			// Does it differ if triggered manually vs via pointcut conditions in a build file
			continue
		}

		// trigger a reload of the configuration
		log.Info("Triggering reload of config after fork")
		t.reload <- struct{}{}
	}
}

// restartableCmd takes the Cmd struct for a process that has been run and creates a new
// one that can be lauched again. Stdin/out will need to be set up again.
func restartableCmd(cmd *exec.Cmd) *exec.Cmd {
	return &exec.Cmd{
		Path:        cmd.Path,
		Args:        cmd.Args,
		Env:         cmd.Env,
		Dir:         cmd.Dir,
		ExtraFiles:  cmd.ExtraFiles,
		SysProcAttr: cmd.SysProcAttr,
	}
}

// Config interface
func (t *tether) UpdateNetworkEndpoint(e *NetworkEndpoint) error {
	defer trace.End(trace.Begin("tether.UpdateNetworkEndpoint"))

	if e == nil {
		return fmt.Errorf("endpoint must be specified")
	}

	if _, ok := t.config.Networks[e.Network.Name]; !ok {
		return fmt.Errorf("network endpoint not found in config")
	}

	t.config.Networks[e.Network.Name] = e
	return nil
}

func (t *tether) Flush() error {
	defer trace.End(trace.Begin("tether.Flush"))

	extraconfig.Encode(t.sink, t.config)
	return nil
}

// killHelper was pulled from toolbox, and that variant should be directed at this
// one eventually
func killHelper(session *SessionConfig) error {
	sig := new(msgs.SignalMsg)
	name := session.StopSignal
	if name == "" {
		name = string(ssh.SIGTERM)
	}

	err := sig.FromString(name)
	if err != nil {
		return err
	}

	num := syscall.Signal(sig.Signum())

	log.Infof("sending signal %s (%d) to %s", sig.Signal, num, session.ID)

	if err := session.Cmd.Process.Signal(num); err != nil {
		return fmt.Errorf("failed to signal %s: %s", session.ID, err)
	}

	return nil
}
