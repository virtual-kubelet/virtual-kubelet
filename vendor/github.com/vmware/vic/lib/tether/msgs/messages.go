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

package msgs

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// All of the messages passed over the ssh channel/global mux are (or will be)
// defined here.

type Message interface {
	// Returns the message name
	RequestType() string

	// Marshalled version of the message
	Marshal() []byte

	// Unmarshal unpacks the message
	Unmarshal([]byte) error
}

// WindowChangeMsg the RFC4254 struct
const WindowChangeReq = "window-change"

type WindowChangeMsg struct {
	Columns  uint32
	Rows     uint32
	WidthPx  uint32
	HeightPx uint32
}

func (wc *WindowChangeMsg) RequestType() string {
	return WindowChangeReq
}

func (wc *WindowChangeMsg) Marshal() []byte {
	return ssh.Marshal(*wc)
}

func (wc *WindowChangeMsg) Unmarshal(payload []byte) error {
	return ssh.Unmarshal(payload, wc)
}

var (
	Signals = map[ssh.Signal]int{
		ssh.SIGABRT: 6,
		ssh.SIGALRM: 14,
		ssh.SIGFPE:  8,
		ssh.SIGHUP:  1,
		ssh.SIGILL:  4,
		ssh.SIGINT:  2,
		ssh.SIGKILL: 9,
		ssh.SIGPIPE: 13,
		ssh.SIGQUIT: 3,
		ssh.SIGSEGV: 11,
		ssh.SIGTERM: 15,
		ssh.SIGUSR1: 10,
		ssh.SIGUSR2: 12,
	}
)

// PingMsg
const PingReq = "ping"
const PingMsg = "Ping"

const UnblockReq = "unblock"
const UnblockMsg = "Unblock"

// VersionMsg
const VersionReq = "version"

type VersionMsg struct {
	Version uint32
}

func (s *VersionMsg) RequestType() string {
	return VersionReq
}

func (s *VersionMsg) Marshal() []byte {
	return ssh.Marshal(*s)
}

func (s *VersionMsg) Unmarshal(payload []byte) error {
	return ssh.Unmarshal(payload, s)
}

// SignalMsg
const SignalReq = "signal"

type SignalMsg struct {
	Signal ssh.Signal
}

func (s *SignalMsg) RequestType() string {
	return SignalReq
}

func (s *SignalMsg) Marshal() []byte {
	return ssh.Marshal(*s)
}

func (s *SignalMsg) Unmarshal(payload []byte) error {
	return ssh.Unmarshal(payload, s)
}

func (s *SignalMsg) Signum() int {
	return Signals[s.Signal]
}

func (s *SignalMsg) FromString(name string) error {
	num, err := strconv.Atoi(name)
	if err == nil {
		for sig, val := range Signals {
			if num == val {
				s.Signal = sig
				return nil
			}
		}
	}

	name = strings.TrimPrefix(strings.ToUpper(name), "SIG")

	s.Signal = ssh.Signal(name)
	_, ok := Signals[s.Signal]
	if !ok {
		return fmt.Errorf("unsupported signal name: %q", name)
	}

	return nil
}

// CloseStdinMsg
const CloseStdinReq = "close-stdin"

// ContainersMsg
const ContainersReq = "container-ids"

type ContainersMsg struct {
	IDs []string
}

func (s *ContainersMsg) RequestType() string {
	return ContainersReq
}

func (s *ContainersMsg) Marshal() []byte {
	return ssh.Marshal(*s)
}

func (s *ContainersMsg) Unmarshal(payload []byte) error {
	return ssh.Unmarshal(payload, s)
}
