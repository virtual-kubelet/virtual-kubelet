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
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"strings"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/pkg/telnet"
	"github.com/vmware/vic/pkg/trace"
)

// handler is the handler struct for the vspc
type handler struct {
	vspc *Vspc
}

func (h *handler) closeHdlr(tc *telnet.Conn) {
	h.vspc.vmManagerMu.Lock()
	defer h.vspc.vmManagerMu.Unlock()

	cvm, exists := h.vspc.cvmFromTelnetConnUnlocked(tc)
	if exists {
		cvm.Lock()
		log.Debugf("(vspc) detected closed connection for VM %s", cvm)
		log.Debugf("(vspc) closing connection to the AttachServer")
		cvm.remoteConn.Close()
		log.Debugf("(vspc) deleting vm records from the vm manager %s", cvm)
		delete(h.vspc.vmManager, cvm.vmUUID)
		cvm.Unlock()
	}
}

// dataHdlr is the telnet data handler
func (h *handler) dataHdlr(w io.Writer, b []byte, tc *telnet.Conn) {
	cvm, exists := h.vspc.cvmFromTelnetConn(tc)
	if !exists {
		// the fsm will sense the closed connection and perform the necessary cleanup
		if tc.UnderlyingConnection() != nil {
			log.Errorln("cannot find a vm associated with this connection.")
			log.Infoln("closing connection")
			tc.UnderlyingConnection().Close()
		}
		return
	}
	cvm.remoteConn.Write(b)

}

// CmdHdlr is the telnet command handler
func (h *handler) cmdHdlr(w io.Writer, b []byte, tc *telnet.Conn) {
	switch {
	case isKnownSuboptions(b):
		log.Infof("vspc received KNOWN-SUBOPTIONS command")
		h.knownSuboptions(w, b)
	case isDoProxy(b):
		log.Infof("vspc received DO-PROXY command")
		h.doProxy(w, b)
	case isVmotionBegin(b):
		log.Infof("vspc received VMOTION-BEGIN command")
		h.vmotionBegin(w, tc, b)
	case isVmotionPeer(b):
		log.Infof("vspc received VMOTION-PEER command")
		h.vmotionPeer(w, b)
	case isVMUUID(b):
		log.Infof("vspc received VMUUID command")
		h.cVMUUID(w, tc, b)
	case isVmotionComplete(b):
		log.Infof("vspc received VMOTION-COMPLETE command")
		h.vmotionComplete(w, tc, b)
	case isVmotionAbort(b):
		log.Infof("vspc received VMOTION-ABORT command")
		h.vmotionAbort(w, tc, b)
	default:
		// log an error here. this should never happen. all commands should be handled appropriately
		log.Errorf("(vspc) received unexpected command")
	}
}

// handleVMUUID handles the telnet vm-uuid response
func (h *handler) cVMUUID(w io.Writer, tc *telnet.Conn, b []byte) {
	defer trace.End(trace.Begin("handling VMUUID"))

	vmuuid := strings.Replace(string(b[3:len(b)-1]), " ", "", -1)
	log.Infof("vmuuid of the connected containerVM: %s", vmuuid)
	// check if there exists another vm with the same vmuuid
	cvm, exists := h.vspc.cVM(vmuuid)
	if !exists || !cvm.isInVMotion() {
		// create a new vm associated with this telnet connection
		cvm = newCVM(tc)
		log.Infof("attempting to connect to the attach server")
		remoteConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", h.vspc.attachSrvrAddr, h.vspc.attachSrvrPort))
		if err != nil {
			log.Errorf("cannot connect to the attach-server: %v", err)
		}
		cvm.remoteConn = remoteConn
		cvm.vmUUID = vmuuid
		h.vspc.addCVM(vmuuid, cvm)
		// relay Reads from the remote system connection to the telnet connection associated with this vm
		go h.vspc.relayReads(cvm, remoteConn)
	} else { //the vm existed before and was shut down or vmotioned
		log.Infof("established second serial-port telnet connection with vm (vmuuid: %s)", cvm.vmUUID)
		cvm.Lock()
		defer cvm.Unlock()
		cvm.prevContainerConn = cvm.containerConn
		cvm.containerConn = tc
	}
}

// handleKnownSuboptions handles the known suboptions telnet command
func (h *handler) knownSuboptions(w io.Writer, b []byte) {
	defer trace.End(trace.Begin("handling KNOWN-SUBOPTIONS"))

	var resp []byte
	suboptions := b[3 : len(b)-1]
	resp = append(resp, []byte{telnet.Iac, telnet.Sb, VmwareExt, KnownSuboptions2}...)
	resp = append(resp, suboptions...)
	resp = append(resp, telnet.Iac, telnet.Se)
	log.Debugf("response to KNOWN-SUBOPTIONS: %v", resp)

	if bytes.IndexByte(suboptions, GetVMVCUUID) != -1 && bytes.IndexByte(suboptions, VMVCUUID) != -1 {
		resp = append(resp, getVMUUID()...)
	}
	w.Write(resp)
}

// handleDoProxy handles the DO-PROXY telnet command
func (h *handler) doProxy(w io.Writer, b []byte) {
	defer trace.End(trace.Begin("handling DO-PROXY"))

	var resp []byte
	resp = append(resp, []byte{telnet.Iac, telnet.Sb, VmwareExt, WillProxy, telnet.Iac, telnet.Se}...)
	log.Debugf("response to DO-PROXY: %v", resp)
	w.Write(resp)
}

// handleVmotionBegin handles the VMOTION-BEGIN telnet command
func (h *handler) vmotionBegin(w io.Writer, tc *telnet.Conn, b []byte) {
	defer trace.End(trace.Begin("handling VMOTION-BEGIN"))

	if cvm, exists := h.vspc.cvmFromTelnetConn(tc); exists {
		cvm.Lock()
		cvm.inVmotion = true
		cvm.Unlock()
		ch := make(chan struct{})
		cvm.vmotionStartedChan <- ch
		<-ch
	}
	seq := b[3 : len(b)-1]
	var escapedSeq []byte
	for _, v := range seq {
		if v == telnet.Iac {
			escapedSeq = append(escapedSeq, telnet.Iac)
		}
		escapedSeq = append(escapedSeq, v)
	}
	secret := make([]byte, 4)
	var escapedSecret []byte
	rand.Read(secret)
	// escaping Iac
	for _, v := range secret {
		if v == telnet.Iac {
			escapedSecret = append(escapedSecret, telnet.Iac)
		}
		escapedSecret = append(escapedSecret, v)
	}
	var resp []byte
	resp = append(resp, []byte{telnet.Iac, telnet.Sb, VmwareExt, VmotionGoahead}...)
	resp = append(resp, escapedSeq...)
	resp = append(resp, escapedSecret...)
	resp = append(resp, telnet.Iac, telnet.Se)
	log.Debugf("response to VMOTION-BEGIN: %v", resp)
	w.Write(resp)
}

// handleVmotionPeer handles the VMOTION-PEER telnet command
func (h *handler) vmotionPeer(w io.Writer, b []byte) {
	defer trace.End(trace.Begin("handling VMOTION-PEER"))

	// cookie is the sequence + secret
	cookie := b[3 : len(b)-1]
	var resp []byte
	resp = append(resp, []byte{telnet.Iac, telnet.Sb, VmwareExt, VmotionPeerOK}...)
	resp = append(resp, cookie...)
	resp = append(resp, telnet.Iac, telnet.Se)
	log.Debugf("response to VMOTION-PEER: %v", resp)
	w.Write(resp)
}

// handleVmotionComplete handles the VMOTION-Complete telnet command
func (h *handler) vmotionComplete(w io.Writer, tc *telnet.Conn, b []byte) {
	defer trace.End(trace.Begin("handling VMOTION-COMPLETE"))

	if cvm, exists := h.vspc.cvmFromTelnetConn(tc); exists {
		cvm.Lock()
		cvm.prevContainerConn = nil
		cvm.inVmotion = false
		cvm.Unlock()
		ch := make(chan struct{})
		cvm.vmotionCompletedChan <- ch
		<-ch
		log.Info("vMotion completed successfully")
	} else {
		log.Errorf("couldnt find previous information of vm after vmotion (vmuuid: %s)", cvm.vmUUID)
	}

}

// handleVmotionAbort handles the VMOTION-abort telnet command
func (h *handler) vmotionAbort(w io.Writer, tc *telnet.Conn, b []byte) {
	defer trace.End(trace.Begin("handling VMOTION-ABORT"))

	log.Errorf("vMotion failed")
	if cvm, exists := h.vspc.cvmFromTelnetConn(tc); exists {
		cvm.Lock()
		cvm.inVmotion = false
		cvm.Unlock()
	}
}
