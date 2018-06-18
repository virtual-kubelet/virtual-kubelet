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

package vspc

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/pkg/telnet"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

/* Vsphere telnet extension constants */
const (
	VmwareExt             byte = 232
	KnownSuboptions1      byte = 0
	KnownSuboptions2      byte = 1
	UnknownSuboptionRcvd1 byte = 2
	UnknownSuboptionRcvd2 byte = 3
	VmotionBegin          byte = 40
	VmotionGoahead        byte = 41
	VmotionNotNow         byte = 43
	VmotionPeer           byte = 44
	VmotionPeerOK         byte = 45
	VmotionComplete       byte = 46
	VmotionAbort          byte = 48
	DoProxy               byte = 70
	WillProxy             byte = 71
	WontProxy             byte = 73
	VMVCUUID              byte = 80
	GetVMVCUUID           byte = 81
	VMName                byte = 82
	GetVMName             byte = 83
	VMBiosUUID            byte = 84
	GetVMBiosUUID         byte = 85
	VMLocationUUID        byte = 86
	GetsVMLocationUUID    byte = 87
	vspcPort              int  = 2377
)
const remoteConnReadDeadline = 1 * time.Second

// cVM is a struct that represents the state of the containerVM
type cVM struct {
	sync.Mutex
	// this is the current connection between the vspc and the containerVM serial port
	containerConn *telnet.Conn
	// in case of vmotion, at some point we will have two telnet connections to the containerVM
	// prevContainerConn is the connection of the source
	// containerConn will be the connection to the destination
	prevContainerConn *telnet.Conn
	// inVmotion is a boolean denoting whether this VM is in a state of vmotion or not
	inVmotion bool

	// remoteConn is the remote-system connection
	// this is the connection between the vspc and the attach-server
	remoteConn net.Conn

	vmUUID               string
	vmotionStartedChan   chan chan struct{}
	vmotionCompletedChan chan chan struct{}
}

// newCVM is the constructor of the VM
func newCVM(tc *telnet.Conn) *cVM {
	return &cVM{
		containerConn:        tc,
		inVmotion:            false,
		vmotionStartedChan:   make(chan chan struct{}),
		vmotionCompletedChan: make(chan chan struct{}),
	}
}

func (cvm *cVM) isInVMotion() bool {
	cvm.Lock()
	defer cvm.Unlock()
	return cvm.inVmotion
}

func (cvm *cVM) String() string {
	return cvm.vmUUID
}

// Vspc is all the vspc singletons
type Vspc struct {
	vmManagerMu sync.Mutex
	vmManager   map[string]*cVM

	*telnet.Server

	attachSrvrAddr string
	attachSrvrPort uint

	doneCh chan bool

	verbose bool
}

// NewVspc is the constructor
func NewVspc() *Vspc {
	defer trace.End(trace.Begin("new vspc"))

	vchIP, err := lookupVCHIP()
	if err != nil {
		log.Fatalf("cannot retrieve vch-endpoint ip: %v", err)
	}
	address := vchIP.String()
	port := constants.SerialOverLANPort

	vspc := &Vspc{
		vmManager: make(map[string]*cVM),

		attachSrvrAddr: "127.0.0.1",
		attachSrvrPort: constants.AttachServerPort,

		doneCh: make(chan bool),
	}
	hdlr := handler{vspc}
	opts := telnet.ServerOpts{
		Addr:       fmt.Sprintf("%s:%d", address, port),
		ServerOpts: []byte{telnet.Binary, telnet.Sga, telnet.Echo},
		ClientOpts: []byte{telnet.Binary, telnet.Sga, VmwareExt},
		Handlers: telnet.Handlers{
			DataHandler:  hdlr.dataHdlr,
			CmdHandler:   hdlr.cmdHdlr,
			CloseHandler: hdlr.closeHdlr,
		},
	}
	vspc.Server = telnet.NewServer(opts)
	vspc.verbose = false

	// load the vchconfig to get debug level
	if src, err := extraconfig.GuestInfoSource(); err == nil {
		extraconfig.Decode(src, &Config)
		if Config.DebugLevel > 2 {
			vspc.verbose = true
		}
	}

	return vspc
}

// Start starts the vspc server
func (vspc *Vspc) Start() {
	defer trace.End(trace.Begin("start vspc"))

	go func() {
		for {
			_, err := vspc.Accept()
			if err != nil {
				log.Errorf("vSPC cannot accept connections: %v", err)
				log.Errorf("vSPC exiting...")
				return
			}
		}
	}()
	log.Infof("vSPC started...")
}

// Stop stops the vspc server
func (vspc *Vspc) Stop() {
	defer trace.End(trace.Begin("stop vspc"))

	vspc.doneCh <- true
}

// cVM returns the VM struct from its uuid
func (vspc *Vspc) cVM(uuid string) (*cVM, bool) {
	vspc.vmManagerMu.Lock()
	defer vspc.vmManagerMu.Unlock()

	if vm, ok := vspc.vmManager[uuid]; ok {
		return vm, true
	}
	return nil, false
}

// addVM adds a VM to the map
func (vspc *Vspc) addCVM(uuid string, cvm *cVM) {
	vspc.vmManagerMu.Lock()
	defer vspc.vmManagerMu.Unlock()

	vspc.vmManager[uuid] = cvm
}

// relayReads reads from the AttachServer connection and relays the data to the telnet connection
func (vspc *Vspc) relayReads(containervm *cVM, conn net.Conn) {
	vmotion := false
	var tmpBuf bytes.Buffer
	for {
		select {
		case ch := <-containervm.vmotionStartedChan:
			vmotion = true
			ch <- struct{}{}
			log.Debugf("vspc started to buffer data coming from the remote system")
		case ch := <-containervm.vmotionCompletedChan:
			vmotion = false
			ch <- struct{}{}
			log.Debugf("vspc stopped buffering data coming from the remote system")
		default:
			b := make([]byte, 4096)
			conn.SetReadDeadline(time.Now().Add(remoteConnReadDeadline))
			var n int
			var err error
			if n, err = conn.Read(b); n > 0 {
				if vspc.verbose {
					log.Debugf("vspc read %d bytes from the  remote system connection", n)
				}
				if !vmotion {
					if tmpBuf.Len() > 0 {
						buf := tmpBuf.Bytes()
						tmpBuf.Reset()
						if err != nil {
							log.Errorf("read error from vspc temporary buffer: %v", err)
						}
						log.Debugf("vspc writing buffered data during vmotion to the containerVM")
						if n, err := containervm.containerConn.WriteData(buf); n == -1 {
							log.Errorf("vspc: RelayReads: %v", err)
							return
						}
					}
					if n, err := containervm.containerConn.WriteData(b[:n]); n == -1 {
						log.Errorf("vspc: RelayReads: %v", err)
						return
					}
				} else {
					tmpBuf.Write(b[:n])
				}
			}
			// if not timeout error exit this goroutine because connection is closed
			if err != nil {
				if err, ok := err.(net.Error); !ok || !err.Timeout() {
					log.Infof("(vspc) remote system connection closed: %v", err)
					return
				}
			}
		}
	}
}

func (vspc *Vspc) cvmFromTelnetConn(tc *telnet.Conn) (*cVM, bool) {
	vspc.vmManagerMu.Lock()
	defer vspc.vmManagerMu.Unlock()

	return vspc.cvmFromTelnetConnUnlocked(tc)
}

// cvmFromTelnetConnUnlocked is the unlocked version of cvmFromTelnetConn
// it expects caller to hold the lock
func (vspc *Vspc) cvmFromTelnetConnUnlocked(tc *telnet.Conn) (*cVM, bool) {
	for _, v := range vspc.vmManager {
		if v.containerConn == tc {
			return v, true
		}
	}
	return nil, false
}
