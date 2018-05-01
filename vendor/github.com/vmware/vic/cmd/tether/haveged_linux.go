// Copyright 2017 VMware, Inc. All Rights Reserved.
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
	"fmt"
	"os"
	"os/exec"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/tether"
)

// Haveged is a tether extension that wraps command
type Haveged struct {
	p    *os.Process
	exec func() (*os.Process, error)
}

// NewHaveged returns a tether.Extension that wraps haveged
func NewHaveged() *Haveged {
	return &Haveged{
		exec: func() (*os.Process, error) {
			args := []string{"/.tether/lib/ld-linux-x86-64.so.2", "--library-path", "/.tether/lib", "/.tether/haveged", "-w", "1024", "-v", "1", "-F"}
			// #nosec: Subprocess launching with variable
			cmd := exec.Command(args[0], args[1:]...)

			log.Infof("Starting haveged with args: %q", args)
			if err := cmd.Start(); err != nil {
				log.Errorf("Starting haveged failed with %q", err.Error())
				return nil, err
			}
			return cmd.Process, nil
		},
	}
}

// Start implementation of the tether.Extension interface
func (h *Haveged) Start() error {
	log.Infof("Starting haveged")

	var err error
	h.p, err = h.exec()
	return err
}

// Stop implementation of the tether.Extension interface
func (h *Haveged) Stop() error {
	log.Infof("Stopping haveged")

	if h.p != nil {
		return h.p.Kill()
	}
	return fmt.Errorf("haveged process is missing")
}

// Reload implementation of the tether.Extension interface
func (h *Haveged) Reload(config *tether.ExecutorConfig) error {
	// haveged doesn't support reloading
	return nil
}
