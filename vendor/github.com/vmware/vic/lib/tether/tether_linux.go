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

package tether

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/kr/pty"

	"github.com/vmware/vic/pkg/trace"
)

const (
	//https://github.com/golang/go/blob/master/src/syscall/zerrors_linux_arm64.go#L919
	SetChildSubreaper = 0x24

	// in sync with lib/apiservers/portlayer/handlers/interaction_handler.go
	// 115200 bps is 14.4 KB/s so use that
	ioCopyBufferSize = 14 * 1024
)

// Mkdev will hopefully get rolled into go.sys at some point
func Mkdev(majorNumber int, minorNumber int) int {
	return (majorNumber << 8) | (minorNumber & 0xff) | ((minorNumber & 0xfff00) << 12)
}

// ReloadConfig signals the current process, which triggers the signal handler
// to reload the config.
func ReloadConfig() error {
	defer trace.End(trace.Begin(""))

	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}

	if err = p.Signal(syscall.SIGHUP); err != nil {
		return err
	}

	return nil
}

// childReaper is used to handle events from child processes, including child exit.
// If running as pid=1 then this means it handles zombie process reaping for orphaned children
// as well as direct child processes.
func (t *tether) childReaper() error {
	signal.Notify(t.incoming, syscall.SIGCHLD)

	/*
	   PR_SET_CHILD_SUBREAPER (since Linux 3.4)
	          If arg2 is nonzero, set the "child subreaper" attribute of the
	          calling process; if arg2 is zero, unset the attribute.  When a
	          process is marked as a child subreaper, all of the children
	          that it creates, and their descendants, will be marked as
	          having a subreaper.  In effect, a subreaper fulfills the role
	          of init(1) for its descendant processes.  Upon termination of
	          a process that is orphaned (i.e., its immediate parent has
	          already terminated) and marked as having a subreaper, the
	          nearest still living ancestor subreaper will receive a SIGCHLD
	          signal and be able to wait(2) on the process to discover its
	          termination status.
	*/
	if _, _, err := syscall.RawSyscall(syscall.SYS_PRCTL, SetChildSubreaper, uintptr(1), 0); err != 0 {
		return err
	}

	log.Info("Started reaping child processes")

	go func() {
		var status syscall.WaitStatus
		flag := syscall.WNOHANG | syscall.WUNTRACED | syscall.WCONTINUED

		for range t.incoming {
			func() {
				// general resiliency
				defer func() {
					if r := recover(); r != nil {
						fmt.Fprintf(os.Stderr, "Recovered in childReaper: %s\n%s", r, debug.Stack())
					}
				}()

				// reap until no more children to process
				for {
					log.Debugf("Inspecting children with status change")

					select {
					case <-t.ctx.Done():
						log.Warnf("Someone called shutdown, returning from child reaper")
						return
					default:
					}

					pid, err := syscall.Wait4(-1, &status, flag, nil)
					// pid 0 means no processes wish to report status
					if pid == 0 || err == syscall.ECHILD {
						log.Debug("No more child processes to reap")
						break
					}

					if err != nil {
						log.Warnf("Wait4 got error: %v\n", err)
						break
					}

					if !status.Exited() && !status.Signaled() {
						log.Debugf("Received notifcation about non-exit status change for %d: %d", pid, status)
						// no reaping or exit handling required
						continue
					}

					exitCode := status.ExitStatus()
					log.Debugf("Reaped process %d, return code: %d", pid, exitCode)

					session, ok := t.removeChildPid(pid)
					if ok {
						log.Debugf("Removed child pid: %d", pid)
						session.Lock()
						session.ExitStatus = exitCode

						t.handleSessionExit(session)
						session.Unlock()
						continue
					}

					ok = t.ops.HandleUtilityExit(pid, exitCode)
					if ok {
						log.Debugf("Remove utility pid: %d", pid)
						continue
					}

					log.Infof("Reaped zombie process PID %d", pid)
				}
			}()
		}
		log.Info("Stopped reaping child processes")
	}()

	return nil
}

func (t *tether) stopReaper() {
	defer trace.End(trace.Begin("Shutting down child reaping"))

	// Ordering is important otherwise we may one goroutine closing, and the other goroutine is trying to write afterwards
	log.Debugf("Removing the signal notifier")
	signal.Reset(syscall.SIGCHLD)

	// just closing the incoming channel is not going to stop the iteration
	// so we use the context cancellation to signal it
	t.cancel()

	log.Debugf("Closing the reapers signal channel")
	close(t.incoming)
}

func (t *tether) triggerReaper() {
	defer trace.End(trace.Begin("Triggering child reaping"))

	t.incoming <- syscall.SIGCHLD
}

func findExecutable(file string, chroot string) error {
	//log.Infof("***** Dumping directory %s", chroot)
	//listDirectory(chroot)
	//log.Infof("***** Dumping directory %s", path.Dir(file))
	//listDirectory(path.Dir(file))
	log.Infof("*** Stating file [%s], chroot [%s]", file, chroot)
	d, err := os.Stat(file)
	if err != nil {
		log.Infof("*** Stating file [%s] failed with error - %s", file, err.Error())

		if chroot != "" {
			file = fmt.Sprintf("%s/%s", chroot, file)
			//log.Infof("***** Dumping directory %s", path.Dir(file))
			//listDirectory(path.Dir(file))
		}
		log.Infof("*** Stating file [%s], chroot [%s]", file, chroot)
		d, err = os.Stat(file)
		if err != nil {
			log.Infof("*** Stating file [%s] failed with error - %s", file, err.Error())
			return err
		}
	}
	log.Infof("*** Stating file [%s], chroot [%s] succeeded", file, chroot)
	if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
		return nil
	}
	return os.ErrPermission
}

// listDirectory logs the directory structure
func listDirectory(path string) error {
	log.Infof("*** Reading directory %s", path)
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Info("*** Reading directory FAILED")
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			log.Infof("*** %s [dir]", file.Name())
		} else {
			log.Infof("*** %s [file]", file.Name())
		}
	}

	return nil
}

// lookPath searches for an executable binary named file in the directories
// specified by the path argument.
// This is a direct modification of the unix os/exec core library impl
func lookPath(file string, env []string, dir string, chroot string) (string, error) {
	// if it starts with a ./ or ../ it's a relative path
	// need to check explicitly to allow execution of .hidden files

	if strings.HasPrefix(file, "./") || strings.HasPrefix(file, "../") {
		file = fmt.Sprintf("%s%c%s", dir, os.PathSeparator, file)
		err := findExecutable(file, chroot)
		if err == nil {
			return filepath.Clean(file), nil
		}
		return "", err
	}

	// check if it's already a path spec
	if strings.Contains(file, "/") {
		err := findExecutable(file, chroot)
		if err == nil {
			return filepath.Clean(file), nil
		}
		return "", err
	}

	// extract path from the env
	var pathenv string
	for _, value := range env {
		if strings.HasPrefix(value, "PATH=") {
			pathenv = value
			break
		}
	}

	pathval := strings.TrimPrefix(pathenv, "PATH=")

	dirs := filepath.SplitList(pathval)
	for _, dir := range dirs {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		path := dir + "/" + file
		if err := findExecutable(path, chroot); err == nil {
			return filepath.Clean(path), nil
		}
	}

	return "", fmt.Errorf("%s: no such executable in PATH", file)
}

func establishPty(session *SessionConfig) error {
	defer trace.End(trace.Begin("initializing pty handling for session " + session.ID))

	// pty.Start creates a process group anyway so no change needed to kill all descendants
	var err error
	session.Pty, err = pty.Start(&session.Cmd)
	if err != nil {
		return err
	}

	session.wait.Add(1)
	go func() {
		_, gerr := io.CopyBuffer(session.Outwriter, session.Pty, make([]byte, ioCopyBufferSize))
		log.Debugf("PTY stdout copy: %s", gerr)

		session.wait.Done()
	}()

	go func() {
		_, gerr := io.CopyBuffer(session.Pty, session.Reader, make([]byte, ioCopyBufferSize))
		log.Debugf("PTY stdin copy: %s", gerr)

		// ensure that an EOT is delivered to the process - this makes the behaviour on EOF at this layer
		// consistent between tty and non-tty cases
		n, gerr := session.Pty.Write([]byte("\x04"))
		if n != 1 || gerr != nil {
			log.Errorf("Failed to write EOT to pty, closing directly: %s", gerr)
			session.Pty.Close()
		}
		log.Debug("Written EOT to pty")
	}()

	return nil
}

func establishNonPty(session *SessionConfig) error {
	defer trace.End(trace.Begin("initializing nonpty handling for session " + session.ID))
	var err error

	// configure a process group so we can kill any descendants
	if session.Cmd.SysProcAttr == nil {
		session.Cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	session.Cmd.SysProcAttr.Setsid = true

	if session.OpenStdin {
		log.Debugf("Setting StdinPipe")
		if session.StdinPipe, err = session.Cmd.StdinPipe(); err != nil {
			log.Errorf("StdinPipe failed with %s", err)
			return err
		}
	}

	log.Debugf("Setting StdoutPipe")
	if session.StdoutPipe, err = session.Cmd.StdoutPipe(); err != nil {
		log.Errorf("Setting StdoutPipe failed with %s", err)
		return err
	}

	log.Debugf("Setting StderrPipe")
	if session.StderrPipe, err = session.Cmd.StderrPipe(); err != nil {
		log.Errorf("Setting StderrPipe failed with %s", err)
		return err
	}

	if session.OpenStdin {
		go func() {
			_, gerr := io.CopyBuffer(session.StdinPipe, session.Reader, make([]byte, ioCopyBufferSize))
			log.Debugf("Reader stdin returned: %s", gerr)

			if gerr == nil {
				if cerr := session.StdinPipe.Close(); cerr != nil {
					log.Errorf("(stdin): Close StdinPipe failed with %s", cerr)
				}
			}
		}()
	}

	// Add 2 for Std{out|err}
	session.wait.Add(2)
	go func() {
		_, gerr := io.CopyBuffer(session.Outwriter, session.StdoutPipe, make([]byte, ioCopyBufferSize))
		log.Debugf("Writer goroutine for stdout returned: %s", gerr)

		if session.StdinPipe != nil {
			log.Debugf("(stdout): Writing zero byte to stdin pipe")
			n, werr := session.StdinPipe.Write([]byte{})
			if n == 0 && werr != nil && werr.Error() == "write |1: bad file descriptor" {
				log.Debugf("(stdout): Closing stdin pipe")
				if cerr := session.StdinPipe.Close(); cerr != nil {
					log.Errorf("Close failed with %s", cerr)
				}
			}
		}
		log.Debugf("Writer goroutine for stdout exiting")

		session.wait.Done()
	}()

	go func() {
		_, gerr := io.CopyBuffer(session.Errwriter, session.StderrPipe, make([]byte, ioCopyBufferSize))
		log.Debugf("Writer goroutine for stderr returned: %s", gerr)

		if session.StdinPipe != nil {
			log.Debugf("(stderr): Writing zero byte to stdin pipe")
			n, werr := session.StdinPipe.Write([]byte{})
			if n == 0 && werr != nil && werr.Error() == "write |1: bad file descriptor" {
				log.Debugf("(stderr): Closing stdin pipe")
				if cerr := session.StdinPipe.Close(); cerr != nil {
					log.Errorf("Close failed with %s", cerr)
				}
			}
		}
		log.Debugf("Writer goroutine for stderr exiting")

		session.wait.Done()
	}()

	return session.Cmd.Start()
}
