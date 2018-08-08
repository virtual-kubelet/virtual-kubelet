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
//
//
//	Client:
//		generate a random uint8 (#)
//		send 2 bytes Syn|#
//
//	Server:
//		generate a random uint8 (&)
//		read at least 2 bytes and make sure Syn|# is received
//		send 3 bytes Ack|#+1|& (or Nak)
//
//  Client:
//	    read at least 3 bytes and make sure Ack|#+1|& is received
//		send 2 bytes Ack|&+1
//
//	Server:
//		read at least 2 bytes and make sure Ack|&+1 is received
//		send 1 byte Ack (or Nak)
//	Client:
//		read at leat 1 byte and make sure Ack is received

package serial

import (
	"fmt"
	"io"
	"math"
	"math/rand"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/pkg/trace"
)

const (
	flagSyn byte = 0x16
	flagAck      = 0x06
	flagNak      = 0x15
)

// HandshakeError should only occure if the protocol between HandshakeServer and HandshakeClient was violated.
type HandshakeError struct {
	msg string
}

func (he *HandshakeError) Error() string {
	return he.msg
}
func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// ReadAtLeastN reads at least l bytes and returns those l bytes or errors
// We get lots of garbage data when we get the initial connection which handshake supposed to clear them and leave the connection in a known state so that the real ssh handshake can start.
// Client and server is looping with different frequencies so client could send multiple requests before server even had a chance to read.
// By getting the last l bytes we are saying that we are not interested with garbage data and also eliminating duplicated flags by only using the last one
func ReadAtLeastN(conn io.ReadWriter, buffer []byte, l int) ([]byte, error) {
	n, err := io.ReadAtLeast(conn, buffer, l)
	if err != nil {
		return nil, err
	}

	// however if we read more than l, it means buffer is not empty
	if n != l {
		buffer = buffer[n-l:]
	}

	return buffer, nil
}

// HandshakeClient establishes connection with the server making sure
// they both are in sync.
func HandshakeClient(conn io.ReadWriter) error {
	if tracing {
		defer trace.End(trace.Begin(""))
	}

	// generate a random pos between [0, math.MaxUint8)
	pos := uint8(rand.Intn(math.MaxUint8))
	buffer := make([]byte, 32*1024)

	// send syn with pos
	log.Debugf("HandshakeClient: Sending syn with pos %d", pos)
	if _, err := conn.Write([]byte{flagSyn, pos}); err != nil {
		log.Errorf("syn: write failed")
		return err
	}

	// read ack with pos+1 and token
	buffer, err := ReadAtLeastN(conn, buffer, 3)
	if err != nil {
		return err
	}

	// extract pos and the token from it
	flag, posack, token := uint8(buffer[0]), uint8(buffer[1]), uint8(buffer[2])
	if flag == flagNak {
		return &HandshakeError{
			msg: "HandshakeClient: Server declined handshake request",
		}
	}
	if flag != flagAck {
		return &HandshakeError{
			msg: fmt.Sprintf("HandshakeClient: Unexpected server response: %#v", flag),
		}
	}

	if posack != pos+1 {
		return &HandshakeError{
			msg: fmt.Sprintf("HandshakeClient: Unexpected ack position: %d, expected %d", posack, pos+1),
		}
	}

	log.Debugf("HandshakeClient: Sending ack with %d", token+1)

	if _, err := conn.Write([]byte{flagAck, token + 1}); err != nil {
		return err
	}

	// last ack packet is 1 byte and could be followed by SSH handshake so read only 1 byteand leave the rest in the net.Conn buffer
	buffer = buffer[:1]
	if _, err := conn.Read(buffer); err != nil {
		return err
	}

	if buffer[0] != flagAck {
		return &HandshakeError{
			msg: fmt.Sprintf("HandshakeClient: Unexpected server response: %#v", flag),
		}
	}

	log.Debug("HandshakeClient: Connection established.")
	return nil
}

// HandshakeServer establishes connection with the client making sure
// they both are in sync.
func HandshakeServer(conn io.ReadWriter) error {
	if tracing {
		defer trace.End(trace.Begin(""))
	}

	// generate a random pos between [0, math.MaxUint8)
	pos := uint8(rand.Intn(math.MaxUint8))
	buffer := make([]byte, 32*1024)

	log.Debug("HandshakeServer: Waiting for incoming syn request...")

	// Sync packet is 2 bytes, however if we read more than 2 it means buffer is not empty and data is not trusted for this sync.
	buffer, err := ReadAtLeastN(conn, buffer, 2)
	if err != nil {
		return err
	}

	// Read 2 bytes, extract flag and the token from it
	flag, token := uint8(buffer[0]), uint8(buffer[1])
	if flag != flagSyn {
		if _, err := conn.Write([]byte{flagNak}); err != nil {
			return err
		}
		return &HandshakeError{
			msg: fmt.Sprintf("Unexpected syn packet: %x", flag),
		}
	}
	log.Debugf("HandshakeServer: Received syn with pos %d. Writing syn-ack with %d and %d", token, token+1, pos)

	// token contains position token that needs to be incremented by one to send it back.
	if _, err := conn.Write([]byte{flagAck, token + 1, pos}); err != nil {
		return err
	}

	// ACK packet is 2 bytes, however if we read more than 2 it means buffer is not empty and data is not trusted for this sync.
	buffer, err = ReadAtLeastN(conn, buffer, 2)
	if err != nil {
		return err
	}

	// Read 2 bytes, extract flag and the token from it
	flag, token = uint8(buffer[0]), uint8(buffer[1])
	if flag != flagAck {
		if _, err := conn.Write([]byte{flagNak}); err != nil {
			return err
		}
		return &HandshakeError{
			msg: fmt.Sprintf("Unexpected syn packet: %x", flag),
		}
	}

	// token should contain incremented pos
	if token != pos+1 {
		if _, err := conn.Write([]byte{flagNak}); err != nil {
			return err
		}
		return &HandshakeError{
			msg: fmt.Sprintf("HandshakeServer: Unexpected position %x, expected: %x", token, pos+1),
		}
	}
	log.Debugf("HandshakeServer: Received ACK with %d.", token)

	// send the last ACK
	if _, err := conn.Write([]byte{flagAck}); err != nil {
		return err
	}

	log.Debug("HandshakeServer: Connection established.")
	return nil
}
