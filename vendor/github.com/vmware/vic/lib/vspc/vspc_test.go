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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/telnet"
)

type dummyWriter struct {
	buf []byte
}

func (w *dummyWriter) Write(b []byte) (int, error) {
	w.buf = b
	return len(b), nil
}

func newTestItem() *Vspc {
	return &Vspc{
		vmManager: make(map[string]*cVM),
		doneCh:    make(chan bool),
	}
}

func TestAddCVM(t *testing.T) {
	vspc := newTestItem()
	cvm := newCVM(nil)
	assert.NotNil(t, cvm)
	vspc.addCVM("dummyid", cvm)
	assert.Equal(t, 1, len(vspc.vmManager))
	for k, v := range vspc.vmManager {
		assert.Equal(t, "dummyid", k)
		assert.Equal(t, cvm, v)
	}
}

func TestGetCVM(t *testing.T) {
	vspc := newTestItem()
	cvm := newCVM(nil)
	assert.NotNil(t, cvm)
	vspc.addCVM("dummyid", cvm)
	storedCVM, exists := vspc.cVM("dummyid")
	assert.Equal(t, true, exists)
	assert.Equal(t, cvm, storedCVM)
	storedCVM, exists = vspc.cVM("dummyidnothere")
	assert.Equal(t, false, exists)
	assert.Nil(t, storedCVM)
}

func TestNewVMStartsWithCorrectVmotion(t *testing.T) {
	vm := newCVM(nil)
	assert.Equal(t, vm.inVmotion, false)
}

func TestHandleSubOptions(t *testing.T) {
	h := &handler{newTestItem()}
	w := &dummyWriter{}
	h.cmdHdlr(w, []byte{telnet.Sb, VmwareExt, KnownSuboptions1, 4, 5, 6, telnet.Se}, nil)
	assert.Equal(t, []byte{telnet.Iac, telnet.Sb, VmwareExt, KnownSuboptions2, 4, 5, 6, telnet.Iac, telnet.Se}, w.buf)
}

func TestHandleDoProxy(t *testing.T) {
	h := &handler{newTestItem()}
	w := &dummyWriter{}
	h.cmdHdlr(w, []byte{telnet.Sb, VmwareExt, DoProxy, telnet.Se}, nil)
	assert.Equal(t, []byte{telnet.Iac, telnet.Sb, VmwareExt, WillProxy, telnet.Iac, telnet.Se}, w.buf)
}

func TestHandleVmotionPeer(t *testing.T) {
	h := &handler{newTestItem()}
	w := &dummyWriter{}
	h.cmdHdlr(w, []byte{telnet.Sb, VmwareExt, VmotionPeer, telnet.Se}, nil)
	assert.Equal(t, []byte{telnet.Iac, telnet.Sb, VmwareExt, VmotionPeerOK, telnet.Iac, telnet.Se}, w.buf)
}
