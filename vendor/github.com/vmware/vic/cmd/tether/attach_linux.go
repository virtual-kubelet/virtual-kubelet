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

package main

import (
	"fmt"
	"net"
	"os"
	"path"
	"syscall"
	"unsafe"

	"golang.org/x/crypto/ssh/terminal"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/tether/msgs"
	"github.com/vmware/vic/pkg/serial"
	"github.com/vmware/vic/pkg/trace"
)

var backchannelMode = os.ModePerm

func setTerminalSpeed(fd uintptr) error {
	var current struct {
		termios syscall.Termios
	}

	// get the current state
	// #nosec: Use of unsafe calls should be audited
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL,
		uintptr(fd),
		syscall.TCGETS,
		uintptr(unsafe.Pointer(&current.termios)),
		0,
		0,
		0,
	); errno != 0 {
		return errno
	}

	// copy it as the future
	future := current.termios

	// unset 9600 bps
	future.Cflag &^= syscall.B9600
	// set them to 115200 bps
	future.Cflag |= syscall.B115200
	future.Ispeed = syscall.B115200
	future.Ospeed = syscall.B115200

	// set the future values
	// #nosec: Use of unsafe calls should be audited
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		syscall.TCSETS,
		uintptr(unsafe.Pointer(&future)),
		0,
		0,
		0,
	); errno != 0 {
		return errno
	}
	return nil
}

func rawConnectionFromSerial() (net.Conn, error) {
	log.Info("opening ttyS0 for backchannel")
	f, err := os.OpenFile(path.Join(pathPrefix, "ttyS0"), os.O_RDWR|os.O_SYNC|syscall.O_NOCTTY, backchannelMode)
	if err != nil {
		detail := fmt.Errorf("failed to open serial port for backchannel: %s", err)
		log.Error(detail)
		return nil, detail
	}

	// set the provided FDs to raw if it's a termial
	// 0 is the uninitialized value for Fd
	if f.Fd() != 0 && terminal.IsTerminal(int(f.Fd())) {
		log.Info("setting terminal to raw mode")
		_, err := terminal.MakeRaw(int(f.Fd()))
		if err != nil {
			return nil, err
		}
	}
	if err := setTerminalSpeed(f.Fd()); err != nil {
		log.Errorf("Setting terminal speed failed with %s", err)
	}

	log.Infof("creating raw connection from ttyS0 (fd=%d)", f.Fd())
	return serial.NewFileConn(f)
}

func resizePty(pty uintptr, winSize *msgs.WindowChangeMsg) error {
	defer trace.End(trace.Begin(""))

	ws := &winsize{uint16(winSize.Rows), uint16(winSize.Columns), uint16(winSize.WidthPx), uint16(winSize.HeightPx)}
	// #nosec: Use of unsafe calls should be audited
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		pty,
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(ws)),
	)
	if errno != 0 {
		return syscall.Errno(errno)
	}
	return nil
}
