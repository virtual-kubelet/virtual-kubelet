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
	"fmt"
	"net"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/d2g/dhcp4"
	"github.com/d2g/dhcp4client"

	"github.com/vmware/vic/lib/dhcp"
	"github.com/vmware/vic/pkg/ip"
	"github.com/vmware/vic/pkg/trace"
)

// Client represents a DHCP client
type Client interface {
	// SetTimeout sets the timeout for a subsequent DHCP request
	SetTimeout(t time.Duration)

	// Request sends a full DHCP request, resulting in a DHCP lease.
	// On a successful lease, returns a DHCP acknowledgment packet
	Request() error

	// Renew renews an existing DHCP lease. Returns a new acknowledgment
	// packet on success.
	Renew() error

	// Release releases an existing DHCP lease.
	Release() error

	// SetParamterRequestList sets the DHCP parameter request list
	// per RFC 2132, section 9.8
	SetParameterRequestList(...byte)

	// LastAck returns the last ack packet from a request or renew operation.
	LastAck() *dhcp.Packet
}

type client struct {
	timeout time.Duration
	id      ID
	params  []byte
	ack     dhcp4.Packet
}

// The default timeout for the client
const defaultTimeout = 10 * time.Second

// NewClient creates a new DHCP client. Note the returned object is not thread-safe.
func NewClient(ifIndex int, hwaddr net.HardwareAddr) (Client, error) {
	defer trace.End(trace.Begin(""))

	id, err := NewID(ifIndex, hwaddr)
	if err != nil {
		return nil, err
	}

	return &client{
		id:      id,
		timeout: defaultTimeout,
	}, nil
}

func (c *client) SetTimeout(t time.Duration) {
	defer trace.End(trace.Begin(""))

	c.timeout = t
}

// Note that the Go runtime sets SA_RESTART for syscalls which retries them automatically if they interrupted.
// However, from the signal man page:
//
//      The following interfaces are never restarted after being interrupted
//      by a signal handler, regardless of the use of SA_RESTART; they always
//      fail with the error EINTR when interrupted by a signal handler:
//
//      * "Input" socket interfaces, when a timeout (SO_RCVTIMEO) has been
//      set on the socket using setsockopt(2): accept(2), recv(2),
//      recvfrom(2), recvmmsg(2) (also with a non-NULL timeout argument),
//      and recvmsg(2).
//
//      * "Output" socket interfaces, when a timeout (SO_RCVTIMEO) has been
//      set on the socket using setsockopt(2): connect(2), send(2),
//      sendto(2), and sendmsg(2).
func withRetry(name string, op func() error) error {
	defer trace.End(trace.Begin(""))

	for {
		if err := op(); err != nil {
			if errno, ok := err.(syscall.Errno); ok {
				if errno == syscall.EAGAIN || errno == syscall.EINTR {
					log.Debugf("retrying %q: errno=%d, error=%s", name, errno, err)
					continue
				}
			}
			return err
		}

		return nil
	}
}

func (c *client) isCompletePacket(p *dhcp.Packet) bool {
	complete := !ip.IsUnspecifiedIP(p.YourIP()) &&
		!ip.IsUnspecifiedIP(p.ServerIP())

	if !complete {
		return false
	}

	for _, param := range c.params {
		switch dhcp4.OptionCode(param) {
		case dhcp4.OptionSubnetMask:
			ones, bits := p.SubnetMask().Size()
			if ones == 0 || bits == 0 {
				return false
			}
		case dhcp4.OptionRouter:
			if ip.IsUnspecifiedIP(p.Gateway()) {
				return false
			}
		case dhcp4.OptionDomainNameServer:
			if len(p.DNS()) == 0 {
				return false
			}
		}
	}

	if p.LeaseTime().Seconds() == 0 {
		return false
	}

	return true
}

func (c *client) discoverPacket(cl *dhcp4client.Client) (dhcp4.Packet, error) {
	defer trace.End(trace.Begin(""))

	dp := cl.DiscoverPacket()
	return c.setOptions(dp)
}

func (c *client) requestPacket(cl *dhcp4client.Client, op *dhcp4.Packet) (dhcp4.Packet, error) {
	defer trace.End(trace.Begin(""))

	rp := cl.RequestPacket(op)
	return c.setOptions(rp)
}

func logDHCPPacketNoOptions(p dhcp4.Packet) {
	log.Debugf("OpCode: %d, HType: %d, HLen: %d, Hops: %d, XId: %+v, Secs: %+v, Flags: %+v, CIAddr: %s, "+
		"YIAddr: %s, SIAddr: %s, GIAddr: %s, CHAddr: %s, Cookie: %+v, Broadcast: %v",
		p.OpCode(), p.HType(), p.HLen(), p.Hops(), p.XId(), p.Secs(), p.Flags(),
		p.CIAddr().String(), p.YIAddr().String(), p.SIAddr().String(), p.GIAddr().String(), p.CHAddr().String(),
		p.Cookie(), p.Broadcast())
}

func logDHCPPacketAndOptions(p dhcp4.Packet, o dhcp4.Options) {
	logDHCPPacketNoOptions(p)
	log.Debugf("Options: %+v", o)
}

func logDHCPPacket(p dhcp4.Packet) {
	o := p.ParseOptions()
	logDHCPPacketAndOptions(p, o)
}

func (c *client) request(cl *dhcp4client.Client) (bool, dhcp4.Packet, error) {
	defer trace.End(trace.Begin(""))

	dp, err := c.discoverPacket(cl)
	if err != nil {
		return false, nil, err
	}

	dp.PadToMinSize()
	if err = cl.SendPacket(dp); err != nil {
		return false, nil, err
	}

	var op dhcp4.Packet
	for {
		op, err = cl.GetOffer(&dp)
		if err != nil {
			return false, nil, err
		}

		if c.isCompletePacket(dhcp.NewPacket([]byte(op))) {
			break
		}
	}

	rp, err := c.requestPacket(cl, &op)
	if err != nil {
		return false, nil, err
	}

	rp.PadToMinSize()
	if err = cl.SendPacket(rp); err != nil {
		return false, nil, err
	}

	ack, err := cl.GetAcknowledgement(&rp)
	if err != nil {
		return false, nil, err
	}

	opts := ack.ParseOptions()
	logDHCPPacketAndOptions(ack, opts)

	if dhcp4.MessageType(opts[dhcp4.OptionDHCPMessageType][0]) == dhcp4.NAK {
		return false, nil, fmt.Errorf("Got NAK from DHCP server")
	}

	return true, ack, nil
}

func (c *client) Request() error {
	defer trace.End(trace.Begin(""))

	log.Debugf("id: %+v", c.id)
	// send the request over a raw socket
	raw, err := dhcp4client.NewPacketSock(c.id.IfIndex)
	if err != nil {
		return err
	}

	rawc, err := dhcp4client.New(
		dhcp4client.Connection(raw),
		dhcp4client.Timeout(c.timeout),
		dhcp4client.HardwareAddr(c.id.HardwareAddr))
	if err != nil {
		return err
	}
	defer rawc.Close()

	success := false
	var p dhcp4.Packet
	err = withRetry("DHCP request", func() error {
		var err error
		success, p, err = c.request(rawc)
		return err
	})

	if err != nil {
		return err
	}

	if !success {
		return fmt.Errorf("failed dhcp request")
	}

	log.Debugf("%+v", p)
	c.ack = p
	return nil
}

func (c *client) newClient() (*dhcp4client.Client, error) {
	defer trace.End(trace.Begin(""))

	ack := dhcp.NewPacket(c.ack)
	conn, err := dhcp4client.NewInetSock(dhcp4client.SetRemoteAddr(net.UDPAddr{IP: ack.ServerIP(), Port: 67}))
	if err != nil {
		return nil, err
	}

	cl, err := dhcp4client.New(dhcp4client.Connection(conn), dhcp4client.Timeout(c.timeout))
	if err != nil {
		return nil, err
	}

	return cl, nil
}

func (c *client) renew(cl *dhcp4client.Client) (dhcp4.Packet, error) {
	defer trace.End(trace.Begin(""))

	rp := cl.RenewalRequestPacket(&c.ack)
	rp, err := c.setOptions(rp)
	if err != nil {
		return nil, err
	}

	rp.PadToMinSize()
	if err = cl.SendPacket(rp); err != nil {
		return nil, err
	}

	newack, err := cl.GetAcknowledgement(&rp)
	if err != nil {
		return nil, err
	}

	opts := newack.ParseOptions()
	if dhcp4.MessageType(opts[dhcp4.OptionDHCPMessageType][0]) == dhcp4.NAK {
		return nil, fmt.Errorf("received NAK from DHCP server")
	}

	return newack, nil
}

func (c *client) Renew() error {
	defer trace.End(trace.Begin(""))

	if c.ack == nil {
		return fmt.Errorf("no ack packet, call Request first")
	}

	cl, err := c.newClient()
	if err != nil {
		return err
	}
	defer cl.Close()

	var newack dhcp4.Packet
	err = withRetry("DHCP renew", func() error {
		var err error
		newack, err = c.renew(cl)
		return err
	})

	if err != nil {
		return err
	}

	c.ack = newack
	return nil
}

func (c *client) Release() error {
	defer trace.End(trace.Begin(""))

	if len(c.ack) == 0 {
		return fmt.Errorf("no ack packet, call Request first")
	}

	cl, err := c.newClient()
	if err != nil {
		return err
	}
	defer cl.Close()

	return withRetry("DHCP release", func() error {
		return cl.Release(c.ack)
	})
}

// SetParamterRequestList sets the DHCP parameter request list
// per RFC 2132, section 9.8
func (c *client) SetParameterRequestList(params ...byte) {
	defer trace.End(trace.Begin(""))

	c.params = make([]byte, len(params))
	copy(c.params, params)
	log.Debugf("c.params=%#v", c.params)
}

// setOptions sets dhcp options on a dhcp packet
func (c *client) setOptions(p dhcp4.Packet) (dhcp4.Packet, error) {
	defer trace.End(trace.Begin(""))

	dirty := false
	opts := p.ParseOptions()

	// the current parameter request list
	rl := opts[dhcp4.OptionParameterRequestList]
	// figure out if there are any new parameters
	for _, p := range c.params {
		if bytes.IndexByte(rl, p) == -1 {
			dirty = true
			rl = append(rl, p)
		}
	}

	opts[dhcp4.OptionParameterRequestList] = rl

	if _, ok := opts[dhcp4.OptionClientIdentifier]; !ok {
		b, err := c.id.MarshalBinary()
		if err != nil {
			return p, err
		}

		opts[dhcp4.OptionClientIdentifier] = b
		dirty = true
	}

	// finally reset the options on the packet, if necessary
	if dirty {
		// strip out all options, and add them back in with the new changed options;
		// this is the only way currently to delete/modify a packet option
		p.StripOptions()
		// have to copy since values in opts (of type []byte) are still pointing into p
		var newp dhcp4.Packet
		newp = make([]byte, len(p))
		copy(newp, p)
		log.Debugf("opts=%#v", opts)
		for o, v := range opts {
			newp.AddOption(o, v)
		}

		p = newp
	}

	return p, nil
}

func (c *client) LastAck() *dhcp.Packet {
	defer trace.End(trace.Begin(""))

	if c.ack == nil {
		return nil
	}

	return dhcp.NewPacket(c.ack)
}
