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

// +build !windows,!darwin

package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/vmware/govmomi/toolbox"
	"github.com/vmware/govmomi/toolbox/vix"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/diag"

	log "github.com/Sirupsen/logrus"
)

// startCommand is the switch for the synthetic commands that are permitted within the appliance.
// This is not intended to allow arbitrary commands to be executed.
// returns:
//  pid: toolbox ProcessManager Process id
//  error
func startCommand(m *toolbox.ProcessManager, r *vix.StartProgramRequest) (int64, error) {
	defer trace.End(trace.Begin(r.ProgramPath))
	var p *toolbox.Process

	switch r.ProgramPath {
	case "enable-ssh":
		p = toolbox.NewProcessFunc(func(ctx context.Context, args string) error {
			err := enableSSH(args)
			// #nosec: Errors unhandled.
			_ = enableShell()
			return err
		})
	case "passwd":
		p = toolbox.NewProcessFunc(func(ctx context.Context, args string) error {
			err := passwd(args)
			// #nosec: Errors unhandled.
			_ = enableShell()
			return err
		})
	case "test-vc-api":
		p = toolbox.NewProcessFunc(func(ctx context.Context, args string) error {
			rc := diag.CheckAPIAvailability(args)
			if rc == diag.VCStatusOK {
				return nil
			}

			return &toolbox.ProcessError{
				Err:      errors.New(diag.UserReadableVCAPITestDescription(rc)),
				ExitCode: int32(rc),
			}
		})
	default:
		return -1, os.ErrNotExist
	}

	return m.Start(r, p)

}

// enableShell changes the root shell from /bin/false to /bin/bash
// We try to ensure the password is not expired via chage, as chsh
// requires an unexpired password to succeed.
func enableShell() error {
	defer trace.End(trace.Begin(""))

	// if reset fails, try the rest anyway
	// #nosec: Errors unhandled.
	resetPasswdExpiry()

	// #nosec: Subprocess launching should be audited
	chsh := exec.Command("/bin/chsh", "-s", "/bin/bash", "root")
	err := chsh.Start()
	if err != nil {
		err := fmt.Errorf("Failed to launch chsh: %s", err)
		log.Error(err)
		return err
	}

	// ignore the error - it's likely raced with child reaper, we just want to make sure
	// that it's exited by the time we pass this point
	// #nosec: Errors unhandled.
	chsh.Wait()

	// confirm the change
	file, err := os.Open("/etc/passwd")
	if err != nil {
		err := fmt.Errorf("Failed to open file to confirm change: %s", err)
		log.Error(err)
		return err
	}

	reader := bufio.NewReader(file)
	line, err := reader.ReadString('\n')
	if err != nil {
		err := fmt.Errorf("Failed to read line from file to confirm change: %s", err)
		log.Error(err)
		return err
	}

	// assert that first line is root
	if !strings.HasPrefix(line, "root") {
		err := fmt.Errorf("Expected line to start with root: %s", line)
		log.Error(err)
		return err
	}

	// assert that first line is root
	if !strings.HasSuffix(line, "/bin/bash\n") {
		err := fmt.Errorf("Expected line to end with /bin/bash: %s", line)
		log.Error(err)
		return err
	}

	log.Info("Attempted to enable the shell for root")

	return nil
}

// passwd sets the password for the root user to that provided as an argument
func passwd(pass string) error {
	defer trace.End(trace.Begin(""))

	// #nosec: Subprocess launching should be audited
	setPasswd := exec.Command("/sbin/chpasswd")
	stdin, err := setPasswd.StdinPipe()
	if err != nil {
		err := fmt.Errorf("Failed to create stdin pipe for chpasswd: %s", err)
		log.Error(err)
		return err
	}

	err = setPasswd.Start()
	if err != nil {
		err := fmt.Errorf("Failed to launch chpasswd: %s", err)
		log.Error(err)
		return err
	}

	_, err = stdin.Write([]byte("root:" + pass))

	// so that we're actively waiting when the process exits, or we'll race (and lose) to child reaper
	go func() {
		// #nosec: Errors unhandled.
		setPasswd.Wait()
	}()

	err = stdin.Close()
	if err != nil {
		err := fmt.Errorf("Failed to close input to chpasswd: %s", err)
		log.Error(err)

		// fire and forget as we're already on error path
		// #nosec: Errors unhandled.
		setPasswd.Process.Kill()

		return err
	}

	log.Info("Attempted to set the password for root")

	return nil
}

// enableSSH receives a key as an argument
func enableSSH(key string) error {
	defer trace.End(trace.Begin(""))

	// basic sanity check for args - we don't bother validating it's a key
	if len(key) != 0 {
		err := os.MkdirAll("/root/.ssh", 0700)
		if err != nil {
			err := fmt.Errorf("unable to create path for keys: %s", err)
			log.Error(err)
			return err
		}

		err = ioutil.WriteFile("/root/.ssh/authorized_keys", []byte(key), 0600)
		if err != nil {
			err := fmt.Errorf("unable to create authorized_keys: %s", err)
			log.Error(err)
			return err
		}
	}

	return startSSH()
}

// startSSH launches the sshd server
func startSSH() error {
	// #nosec: Subprocess launching should be audited
	c := exec.Command("/usr/bin/systemctl", "start", "sshd")

	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b

	if err := c.Start(); err != nil {
		return err
	}

	go func() {
		// because init is explicitly reaping child processes we cannot use simple
		// exec commands to gather status
		// #nosec: Errors unhandled.
		_ = c.Wait()
		log.Info("Attempted to start ssh service:\n %s", b)
	}()

	return nil
}

func resetPasswdExpiry() error {
	defer trace.End(trace.Begin(""))

	// add just enough time for the password not to be expired
	// if the user wants more time they can actually change the password
	// This will expire in at most 1 day, perhaps sooner depending on local time
	// NB: Format is example based - that's the reference time format
	expireDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// #nosec: Subprocess launching should be audited
	chage := exec.Command("/bin/chage", "-M", "1", "-d", expireDate, "root")
	err := chage.Start()
	if err != nil {
		err := fmt.Errorf("Failed to launch chage: %s", err)
		log.Error(err)
		return err
	}

	// ignore the error - it's likely raced with child reaper, we just want to make sure
	// that it's exited by the time we pass this point
	// #nosec: Errors unhandled.
	chage.Wait()

	log.Infof("Attempted reset of password expiry: /bin/chage -M 1 -d %s root", expireDate)
	return nil
}
