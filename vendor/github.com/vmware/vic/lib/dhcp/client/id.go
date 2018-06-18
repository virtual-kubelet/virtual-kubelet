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
	"bytes"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/dchest/siphash"
	"github.com/google/uuid"

	"github.com/vmware/vic/lib/system"
)

var Sys system.System

func init() {
	Sys = system.New()
}

// Duid is a vendor based DUID per https://tools.ietf.org/html/rfc3315#section-9.3
type Duid struct {
	Type uint16
	PEN  uint32
	ID   uint64
}

// Iaid is an opaque 32-bit identifier unique to a network interface; see https://tools.ietf.org/html/rfc4361#section-6.1
type Iaid []byte

// ID is a DHCPv4 client ID
type ID struct {
	Type uint8
	Iaid Iaid
	Duid Duid

	IfIndex      int
	HardwareAddr net.HardwareAddr
}

// VMwarePEN is VMware's PEN (Private Enterprise Number); see http://www.iana.org/assignments/enterprise-numbers/enterprise-numbers
const VMwarePEN uint32 = 6876

// DuidEn is the DUID type; see https://tools.ietf.org/html/rfc3315#section-9.3
const DuidEn uint16 = 2

var key = []byte{0xc4, 0xeb, 0x38, 0x9e, 0x4e, 0xd9, 0x48, 0x12, 0x93, 0xa6, 0xb7, 0x0b, 0x9b, 0x07, 0xcc, 0x2b}

// NewID generates a DHCPv4 client ID, per https://tools.ietf.org/html/rfc4361#section-6.1
func NewID(ifindex int, hw net.HardwareAddr) (ID, error) {
	iaid := makeIaid(ifindex, hw)
	duid, err := makeDuid()
	if err != nil {
		return ID{}, err
	}

	return ID{
		Type:         255,
		Iaid:         iaid,
		Duid:         duid,
		IfIndex:      ifindex,
		HardwareAddr: hw,
	}, nil
}

// MarshalBinary implements the BinaryMarshaler interface
func (i ID) MarshalBinary() ([]byte, error) {
	var b []byte
	b = append(b, i.Type)
	ib, err := i.Iaid.MarshalBinary()
	if err != nil {
		return nil, err
	}
	b = append(b, ib...)
	db, err := i.Duid.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return append(b, db...), nil
}

// MarshalBinary implements the BinaryMarshaler interface
func (ia Iaid) MarshalBinary() ([]byte, error) {
	return ia[:], nil
}

// MarshalBinary implements the BinaryMarshaler interface
func (d Duid) MarshalBinary() ([]byte, error) {
	b := new(bytes.Buffer)
	// #nosec: Errors unhandled.
	binary.Write(b, binary.BigEndian, d.Type)
	// #nosec: Errors unhandled.
	binary.Write(b, binary.BigEndian, d.PEN)
	// #nosec: Errors unhandled.
	binary.Write(b, binary.BigEndian, d.ID)
	return b.Bytes(), nil
}

// makeIaid constructs a new IAID. Ported from systemd's
// implementation here: https://github.com/systemd/systemd/blob/master/src/libsystemd-network/dhcp-identifier.c
func makeIaid(ifindex int, hw net.HardwareAddr) Iaid {
	h := siphash.New(key)
	// #nosec: Errors unhandled.
	h.Write(hw)
	id := h.Sum64()

	// fold into 32 bits
	iaid := make(Iaid, 4)
	binary.BigEndian.PutUint32(iaid, uint32(id&0xffffffff)^uint32(id>>32))
	return iaid
}

func getMachineID() ([]byte, error) {
	id := Sys.UUID
	if id == "" {
		return nil, fmt.Errorf("could not get machine id")
	}

	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	return uid.MarshalBinary()
}

// makeDuid constructs a new DUID. Adapted from systemd's implemenation here: https://github.com/systemd/systemd/blob/master/src/libsystemd-network/dhcp-identifier.c
func makeDuid() (Duid, error) {
	id, err := getMachineID()
	if err != nil {
		return Duid{}, err
	}

	h := siphash.New(key)
	// #nosec: Errors unhandled.
	h.Write(id)
	return Duid{Type: DuidEn, PEN: VMwarePEN, ID: h.Sum64()}, nil
}
