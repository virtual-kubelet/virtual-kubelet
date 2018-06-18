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
package client

import (
	"bufio"
	"errors"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"

	"io/ioutil"

	log "github.com/Sirupsen/logrus"
	"github.com/d2g/dhcp4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/system"
)

var dummyHWAddr net.HardwareAddr = []byte{0x6, 0x0, 0x0, 0x0, 0x0, 0x0}

func TestMain(m *testing.M) {
	Sys = system.System{
		UUID: uuid.New().String(),
	}

	os.Exit(m.Run())
}

func TestSetOptions(t *testing.T) {
	id, err := NewID(0, dummyHWAddr)
	assert.NoError(t, err)
	c := &client{
		id: id,
	}
	p := dhcp4.NewPacket(dhcp4.BootRequest)

	var tests = []struct {
		prl []byte
	}{
		{
			prl: []byte{
				byte(dhcp4.OptionSubnetMask),
				byte(dhcp4.OptionRouter),
			},
		},
		{
			prl: []byte{
				byte(dhcp4.OptionSubnetMask),
				byte(dhcp4.OptionRouter),
				byte(dhcp4.OptionNameServer),
			},
		},
	}

	for _, te := range tests {
		c.SetParameterRequestList(te.prl...)
		p, err := c.setOptions(p)
		assert.NoError(t, err)
		assert.NotEmpty(t, p)

		opts := p.ParseOptions()

		prl := opts[dhcp4.OptionParameterRequestList]
		assert.NotNil(t, prl)
		assert.EqualValues(t, te.prl, prl)

		// packet should have client id set
		cid := opts[dhcp4.OptionClientIdentifier]
		assert.NotNil(t, cid)
		b, _ := id.MarshalBinary()
		assert.EqualValues(t, cid, b)
	}
}

func TestWithRetry(t *testing.T) {
	errors := []error{
		syscall.Errno(syscall.EAGAIN),
		syscall.Errno(syscall.EINTR),
		errors.New("fail"),
	}

	i := 0

	err := withRetry("test fail", func() error {
		e := errors[i]
		i++
		return e
	})

	if err != errors[len(errors)-1] {
		t.Errorf("err=%s", err)
	}

	err = withRetry("test ok", func() error {
		return nil
	})

	if err != nil {
		t.Errorf("err=%s", err)
	}
}

const packetStr string = "OpCode: 1, HType: 2, HLen: 14, Hops: 3, XId: [1 2 3 4], Secs: [0 0], Flags: [5 6], " +
	"CIAddr: 1.2.3.4, YIAddr: 0.0.0.0, SIAddr: 0.0.0.0, GIAddr: 0.0.0.0, " + "" +
	"CHAddr: 01:02:03:04:05:06:07:08:09:0a:0b:0c:0d:0e, " +
	"Cookie: [7 8 9 10], Broadcast: false"

const optionStr string = "Options: map"

func TestLog(t *testing.T) {
	id, err := NewID(0, dummyHWAddr)
	assert.NoError(t, err)
	c := &client{
		id: id,
	}

	p := dhcp4.NewPacket(dhcp4.BootRequest)
	p.SetHType(2)
	p.SetHops(3)
	p.SetXId([]byte{1, 2, 3, 4})
	p.SetFlags([]byte{5, 6})
	p.SetCookie([]byte{7, 8, 9, 10})
	p.SetCIAddr([]byte{1, 2, 3, 4})
	p.SetCHAddr([]byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe})

	var opts = struct {
		prl []byte
	}{
		prl: []byte{
			byte(dhcp4.OptionSubnetMask),
			byte(dhcp4.OptionRouter),
			byte(dhcp4.OptionNameServer),
		},
	}

	f, err := ioutil.TempFile("", "DhcpClient-")
	defer os.Remove(f.Name())

	if err == nil {
		log.SetOutput(f)
		log.SetLevel(log.DebugLevel)
	}

	c.SetParameterRequestList(opts.prl...)
	p, err = c.setOptions(p)
	assert.NoError(t, err)
	assert.NotEmpty(t, p)
	// Log packet
	logDHCPPacket(p)
	// Log packet and options
	o := p.ParseOptions()
	logDHCPPacketAndOptions(p, o)
	res := verifyLog(f)
	assert.True(t, res)
}

func verifyLog(lFile *os.File) bool {
	f, err := os.Open(lFile.Name())
	if err != nil {
		return false
	}

	countBody := 0
	countOptions := 0
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, packetStr) {
			countBody++
		} else if strings.Contains(line, optionStr) {
			countOptions++
		}
	}

	return countBody == 2 && countOptions == 2
}
