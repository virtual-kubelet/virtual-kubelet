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

package telnet

import log "github.com/Sirupsen/logrus"

type state int

const (
	dataState state = iota
	optionNegotiationState
	cmdState
	subnegState
	subnegEndState
	errorState
)

type fsm struct {
	curState state
	tc       *Conn
}

func newFSM() *fsm {
	f := &fsm{
		curState: dataState,
	}
	return f
}

func (fsm *fsm) start() {
	defer func() {
		log.Infof("FSM closed")
	}()
	for {
		b := make([]byte, 4096)
		n, err := fsm.readFromRawConnection(b)
		if n > 0 {
			for i := 0; i < n; i++ {
				ch := b[i]
				ns := fsm.nextState(ch)
				fsm.curState = ns
			}
		}
		if err != nil {
			log.Debugf("connection read: %v", err)
			fsm.tc.close()
			break
		}
	}
}

func (fsm *fsm) readFromRawConnection(b []byte) (int, error) {
	return fsm.tc.conn.Read(b)
}

// this function returns what the next state is and performs the appropriate action
func (fsm *fsm) nextState(ch byte) state {
	var nextState state
	b := []byte{ch}
	switch fsm.curState {
	case dataState:
		if ch != Iac {
			fsm.tc.dataRW.Write(b)
			fsm.tc.dataWrittenCh <- true
			nextState = dataState
		} else {
			nextState = cmdState
		}

	case cmdState:
		if ch == Iac { // this is an escaping of IAC to send it as data
			fsm.tc.dataRW.Write(b)
			fsm.tc.dataWrittenCh <- true
			nextState = dataState
		} else if ch == Do || ch == Dont || ch == Will || ch == Wont {
			fsm.tc.cmdBuffer.WriteByte(ch)
			nextState = optionNegotiationState
		} else if ch == Sb {
			fsm.tc.cmdBuffer.WriteByte(ch)
			nextState = subnegState
		} else { // anything else
			fsm.tc.cmdBuffer.WriteByte(ch)
			fsm.tc.cmdHandlerWrapper(fsm.tc.handlerWriter, &fsm.tc.cmdBuffer)
			fsm.tc.cmdBuffer.Reset()
			nextState = dataState
		}
	case optionNegotiationState:
		fsm.tc.cmdBuffer.WriteByte(ch)
		opt := ch
		cmd := fsm.tc.cmdBuffer.Bytes()[0]
		fsm.tc.optCallback(cmd, opt)
		fsm.tc.cmdBuffer.Reset()
		nextState = dataState
	case subnegState:
		if ch == Iac {
			nextState = subnegEndState
		} else {
			nextState = subnegState
			fsm.tc.cmdBuffer.WriteByte(ch)
		}
	case subnegEndState:
		if ch == Se {
			fsm.tc.cmdBuffer.WriteByte(ch)
			fsm.tc.cmdHandlerWrapper(fsm.tc.handlerWriter, &fsm.tc.cmdBuffer)
			fsm.tc.cmdBuffer.Reset()
			nextState = dataState
		} else if ch == Iac { // escaping IAC
			nextState = subnegState
			fsm.tc.cmdBuffer.WriteByte(ch)
		} else {
			nextState = errorState
		}
	case errorState:
		nextState = dataState
		log.Infof("Finite state machine is in an error state. This should not happen for correct telnet protocol syntax")
	}
	return nextState
}
