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

package dns

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"

	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/portlayer/network"
	"github.com/vmware/vic/pkg/trace"

	mdns "github.com/miekg/dns"
)

const (
	DefaultIP        = "127.0.0.1"
	DefaultPort      = 53
	DefaultTTL       = 600 * time.Second
	DefaultCacheSize = 1024
	DefaultTimeout   = 4 * time.Second
	hexDigit         = "0123456789abcdef"
)

var (
	options = ServerOptions{}
	random  *rand.Rand
)

func init() {
	random = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// ServerOptions represents the server options
type ServerOptions struct {
	IP        string
	Port      int
	Interface string

	Nameservers flagMultipleVar

	Timeout time.Duration

	TTL       time.Duration
	CacheSize int

	Debug bool
}

// Server represents udp/tcp server and clients
type Server struct {
	ServerOptions

	// used for serving dns
	udpserver *mdns.Server
	udpconn   *net.UDPConn

	tcpserver *mdns.Server
	tcplisten *net.TCPListener

	// used for forwarding queries
	udpclient *mdns.Client
	tcpclient *mdns.Client

	// used for speeding up external lookups
	cache *Cache
	wg    *sync.WaitGroup
}

type flagMultipleVar []string

func (i *flagMultipleVar) String() string {
	return fmt.Sprint(*i)
}

func (i *flagMultipleVar) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// NewServer returns a new Server
func NewServer(options ServerOptions) *Server {
	var err error

	// Default TTL
	if options.TTL == 0 {
		options.TTL = DefaultTTL
	}

	// Default port
	if options.Port == 0 {
		options.Port = DefaultPort
	}

	// Default nameservers
	if len(options.Nameservers) == 0 {
		options.Nameservers = resolvconf()
	}

	// Default cache size
	if options.CacheSize == 0 {
		options.CacheSize = DefaultCacheSize
	}

	// Default timeout
	if options.Timeout == 0 {
		options.Timeout = DefaultTimeout
	}

	server := &Server{
		ServerOptions: options,
		cache:         NewCache(CacheOptions{options.CacheSize, options.TTL}),
		wg:            new(sync.WaitGroup),
	}

	udpaddr := &net.UDPAddr{
		IP:   net.ParseIP(server.IP),
		Port: server.Port,
	}

	server.udpconn, err = net.ListenUDP("udp", udpaddr)
	if err != nil {
		log.Errorf("ListenUDP failed %s", err)
		return nil
	}

	tcpaddr := &net.TCPAddr{
		IP:   net.ParseIP(server.IP),
		Port: server.Port,
	}

	server.tcplisten, err = net.ListenTCP("tcp", tcpaddr)
	if err != nil {
		log.Errorf("ListenTCP failed %s", err)
		return nil
	}

	server.udpclient = &mdns.Client{
		Net:            "udp",
		ReadTimeout:    server.Timeout,
		WriteTimeout:   server.Timeout,
		SingleInflight: true,
	}

	server.tcpclient = &mdns.Client{
		Net:            "tcp",
		ReadTimeout:    server.Timeout,
		WriteTimeout:   server.Timeout,
		SingleInflight: true,
	}

	return server
}

// Addr returns the ip:port of the server
func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.IP, s.Port)
}

func respServerFailure(w mdns.ResponseWriter, r *mdns.Msg) error {
	m := new(mdns.Msg)
	m.SetRcode(r, mdns.RcodeServerFailure)
	// Does not matter if this write fails
	return w.WriteMsg(m)
}

func respNotImplemented(w mdns.ResponseWriter, r *mdns.Msg) error {
	m := &mdns.Msg{
		MsgHdr: mdns.MsgHdr{
			Authoritative:      false,
			RecursionDesired:   false,
			RecursionAvailable: false,
			Rcode:              mdns.RcodeNotImplemented,
		},
		Compress: true,
	}
	m.SetReply(r)

	if err := w.WriteMsg(m); err != nil {
		log.Errorf("Error writing RcodeNotImplemented response, %s", err)
		return err
	}
	return nil
}

func resolvconf() []string {
	var servers []string

	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// skip comments
		if len(line) > 0 && (line[0] == ';' || line[0] == '#') {
			continue
		}
		f := strings.SplitN(line, " ", 2)

		if len(f) < 1 {
			continue
		}

		if f[0] == "nameserver" {
			if len(f) > 1 && len(servers) < 3 { // small, but the standard limit
				// One more check: make sure server name is
				// just an IP address.  Otherwise we need DNS
				// to look it up.
				if net.ParseIP(f[1]) != nil {
					servers = append(servers, f[1])
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil
	}
	return servers
}

// SeenBefore returns the cached response
func (s *Server) SeenBefore(w mdns.ResponseWriter, r *mdns.Msg) (bool, error) {
	defer trace.End(trace.Begin(r.String()))

	// Do we have it in the cache
	if m := s.cache.Get(r); m != nil {
		log.Debugf("Cache hit for %q", r.String())

		// Overwrite the ID with the request's ID
		m.Id = r.Id
		m.Compress = true
		m.Truncated = false

		if err := w.WriteMsg(m); err != nil {
			log.Errorf("Error writing response: %q", err)
			return true, err
		}
		return true, nil
	}
	return false, nil
}

// HandleForwarding forwards a request to the nameservers and returns the response
func (s *Server) HandleForwarding(w mdns.ResponseWriter, r *mdns.Msg) (bool, error) {
	defer trace.End(trace.Begin(r.String()))

	var m *mdns.Msg
	var err error
	var try int

	if len(s.Nameservers) == 0 {
		log.Errorf("No nameservers defined, can not forward")
		return false, respServerFailure(w, r)
	}

	// which protocol are they talking
	tcp := false
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		tcp = true
	}

	// Use request ID for "random" nameserver selection.
	nsid := int(r.Id) % len(s.Nameservers)

Redo:
	nameserver := s.Nameservers[nsid]
	if i := strings.Index(nameserver, ":"); i < 0 {
		nameserver += ":53"
	}

	if tcp {
		m, _, err = s.tcpclient.Exchange(r, nameserver)
	} else {
		m, _, err = s.udpclient.Exchange(r, nameserver)
	}
	if err != nil {
		// Seen an error, this can only mean, "server not reached", try again but only if we have not exausted our nameservers.
		if try < len(s.Nameservers) {
			try++
			nsid = (nsid + 1) % len(s.Nameservers)
			goto Redo
		}

		log.Errorf("Failure to forward request: %q", err)
		return false, respServerFailure(w, r)
	}

	// We have a response so cache it
	s.cache.Add(m)

	m.Compress = true
	if err := w.WriteMsg(m); err != nil {
		log.Errorf("Error writing response: %q", err)
		return true, err
	}
	return true, nil
}

// lookupByAlias looks up a container by alias, given the requesting container
// and the network of the incoming request. It first does a container-scoped
// alias search, followed by a network-scoped aliased search for alias. The
// result is a collection of endpoints for the matching container that are
// only on the same network as the requesting container.
func lookupByAlias(netCtx *network.Context, scope *network.Scope, reqc *network.Container, alias string) []*network.Endpoint {
	// container specific alias search
	cons := netCtx.ContainersByAlias(network.ScopedAliasName(scope.Name(), reqc.Name(), alias))
	if len(cons) == 0 {
		// scope-wide alias search
		cons = netCtx.ContainersByAlias(network.ScopedAliasName(scope.Name(), "", alias))
	}

	if len(cons) == 0 {
		return nil
	}

	var eps []*network.Endpoint
	for _, c := range cons {
		if e := c.Endpoint(scope); e != nil {
			eps = append(eps, e)
		}
	}

	return eps
}

// lookupByName looks up a container by name given the requesting container. It returns a collection
// of endpoints on networks that both the requesting and matching container share.
func lookupByName(netCtx *network.Context, reqc *network.Container, name string) []*network.Endpoint {
	var eps []*network.Endpoint
	if m := netCtx.Container(name); m != nil {
		// look if networks overlap for the requesting container and the
		// matched container
		for _, ec := range reqc.Endpoints() {
			if em := m.Endpoint(ec.Scope()); em != nil {
				eps = append(eps, em)
			}
		}
	}

	return eps
}

// HandleVIC returns a response to a container name/id request
func (s *Server) HandleVIC(w mdns.ResponseWriter, r *mdns.Msg) (bool, error) {
	defer trace.End(trace.Begin(r.String()))

	question := r.Question[0]

	netCtx := network.DefaultContext
	if netCtx == nil {
		log.Errorf("DefaultContext is not initialized")
		return false, fmt.Errorf("DefaultContext is not initialized")
	}

	clientIP, _, err := net.SplitHostPort(w.RemoteAddr().String())
	if err != nil {
		log.Errorf("SplitHostPort failed: %q", err)
		return false, err
	}

	log.Debugf("RemoteAddr: %s", clientIP)
	ip := net.ParseIP(clientIP)

	var name string
	var domain string

	name = strings.TrimSuffix(question.Name, ".")
	// Do we have a domain?
	i := strings.IndexRune(name, '.')
	if i >= 0 {
		name, domain = name[:i], name[i+1:]
	}

	// get the requesting container's endpoint
	e := netCtx.ContainerByAddr(ip)
	if e == nil {
		return false, fmt.Errorf("Could not find requesting container with ip %s", ip)
	}

	if domain != "" && e.Scope().Name() != domain {
		return false, fmt.Errorf("Intra-scope request for container %s in %s from %s", name, domain, e.Scope().Name())
	}

	eps := lookupByAlias(netCtx, e.Scope(), e.Container(), name)
	if len(eps) == 0 {
		// lookup container by name
		eps = lookupByName(netCtx, e.Container(), name)
	}

	if len(eps) == 0 {
		log.Debugf("Can't find the container: %q", name)
		return false, fmt.Errorf("Can't find the container: %q", name)
	}

	// FIXME: Add AAAA when we support it
	answer := make([]mdns.RR, len(eps))
	// shuffle eps (Fisher–Yates shuffle)
	for i := len(eps) - 1; i > 0; i-- {
		j := random.Intn(i + 1)
		eps[i], eps[j] = eps[j], eps[i]
	}

	for i, e := range eps {
		if e.IP().IsUnspecified() {
			return false, fmt.Errorf("No ip for container %q", name)
		}

		answer[i] = &mdns.A{
			Hdr: mdns.RR_Header{
				Name:   question.Name,
				Rrtype: mdns.TypeA,
				Class:  mdns.ClassINET,
				Ttl:    uint32(DefaultTTL.Seconds()),
			},
			A: e.IP(),
		}
	}

	// Start crafting reply msg
	m := &mdns.Msg{
		MsgHdr: mdns.MsgHdr{
			Authoritative:      true,
			RecursionAvailable: true,
		},
		Compress: true,
	}
	m.SetReply(r)

	m.Answer = append(m.Answer, answer...)

	// Which protocol we are talking
	tcp := false
	if _, ok := w.LocalAddr().(*net.TCPAddr); ok {
		tcp = true
	}

	// 512 byte payload guarantees that DNS packets can be reassembled if fragmented in transit.
	bufsize := 512

	// With EDNS0 in use a larger payload size can be specified.
	if o := r.IsEdns0(); o != nil {
		bufsize = int(o.UDPSize())
	}

	// Make sure we are not smaller than 512
	if bufsize < 512 {
		bufsize = 512
	}

	// With TCP we can send up to 64K
	if tcp {
		bufsize = mdns.MaxMsgSize - 1
	}

	// Trim the answer RRs one by one till the whole message fits within the reply size
	if m.Len() > bufsize {
		if tcp {
			m.Truncated = true
		}

		for m.Len() > bufsize {
			m.Answer = m.Answer[:len(m.Answer)-1]
		}
	}

	if err := w.WriteMsg(m); err != nil {
		log.Errorf("Error writing response, %s", err)
		return true, err
	}
	w.Close()

	return true, nil
}

// ServeDNS implements the handler interface
func (s *Server) ServeDNS(w mdns.ResponseWriter, r *mdns.Msg) {
	defer trace.End(trace.Begin(r.String()))

	if r == nil || len(r.Question) == 0 {
		return
	}

	// Reject multi-question query
	if len(r.Question) != 1 {
		log.Errorf("Rejected multi-question query")

		respServerFailure(w, r)
		return
	}
	q := r.Question[0]

	// Reject non-INET type query
	if q.Qclass != mdns.ClassINET {
		log.Errorf("Rejected non-inet query")

		respNotImplemented(w, r)
		return
	}

	// Reject ANY type query
	if q.Qtype == mdns.TypeANY {
		log.Errorf("Rejected ANY query")

		respNotImplemented(w, r)
		return
	}

	// Check VIC first
	// Currently VIC can only answer ipv4 "A" queries
	if q.Qtype == mdns.TypeA {
		ok, err := s.HandleVIC(w, r)
		if ok {
			if err != nil {
				log.Errorf("HandleVIC returned: %q", err)
			}
			return
		}
	}

	// Do we have the response in our cache?
	ok, err := s.SeenBefore(w, r)
	if ok {
		if err != nil {
			log.Errorf("SeenBefore returned: %q", err)
		}
		return
	}

	// Then forward
	ok, err = s.HandleForwarding(w, r)
	if ok {
		if err != nil {
			log.Errorf("HandleForwarding returned: %q", err)
		}
		return
	}

}

// Start starts the DNS server
func (s *Server) Start() {
	// Call BindToDevice if IP is empty and Interface is set
	if s.IP == "" && s.Interface != "" {
		uf, err := s.udpconn.File()
		if err != nil {
			log.Errorf("Getting the fd failed with: %s", err)
		} else {
			// BindToDevice binds the socket associated with fd to device.
			if err := BindToDevice(int(uf.Fd()), s.Interface); err != nil {
				log.Errorf("Calling BindToDevice failed with: %s", err)
			}
		}
	}

	udpserver := &mdns.Server{
		Handler:    s,
		PacketConn: s.udpconn,
	}
	s.udpserver = udpserver

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		udpserver.ActivateAndServe()
		log.Debugf("UDP server exited")
	}()
	log.Infof("Ready for queries on udp://%s", s.Addr())

	// Call BindToDevice if IP is empty and Interface is set
	if s.IP == "" && s.Interface != "" {
		tf, err := s.tcplisten.File()
		if err != nil {
			log.Errorf("Getting the fd failed with: %s", err)
		} else {
			// BindToDevice binds the socket associated with fd to device.
			if err := BindToDevice(int(tf.Fd()), s.Interface); err != nil {
				log.Errorf("Calling BindToDevice failed with: %s", err)
			}
		}
	}

	tcpserver := &mdns.Server{Handler: s, Listener: s.tcplisten}
	s.tcpserver = tcpserver

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		tcpserver.ActivateAndServe()
		log.Debugf("TCP server exited")
	}()
	log.Infof("Ready for queries on tcp://%s", s.Addr())

}

// Stop stops the DNS server gracefully
func (s *Server) Stop() {
	if s.udpserver != nil {
		log.Debugf("Shutting down udpserver")
		s.udpserver.Shutdown()
	}
	s.udpconn = nil
	s.udpserver = nil

	if s.tcpserver != nil {
		log.Debugf("Shutting down tcpserver")
		s.tcpserver.Shutdown()
	}
	s.tcplisten = nil
	s.tcpserver = nil
}

// Wait block until wg returns
func (s *Server) Wait() {
	s.wg.Wait()
}

// SetOfDomains is a type for storing string-type domain names as an unsorted set
//     var f SetOfDomains
//     f = make(map[string]bool)
// Store in the set
//     f["foo.com"] = true
// then to check to see if something is in the 'set':
//     if f["foo.com"] {
type SetOfDomains map[string]bool

// ReverseLookup returns a set of FQDNs for ipAddr from nameservers in /etc/resolv.conf
// /etc/hosts and /etc/nsswitch.conf are ignored by this function
func ReverseLookup(ipAddr string) (domains SetOfDomains) {
	domains = make(map[string]bool)

	address, err := reverseaddr(ipAddr)
	if err != nil {
		log.Errorf("%s", err)
		return
	}

	nameservers := resolvconf()
	for _, n := range nameservers {
		dnsClient := new(mdns.Client)
		msg := new(mdns.Msg)

		msg.SetQuestion(address, mdns.TypePTR)
		r, _, err := dnsClient.Exchange(msg, n+":53")
		if err != nil {
			log.Warnf("got error \"%s\" from %s", err, n)
			continue
		}

		if len(r.Answer) == 0 {
			log.Warnf("no reply from %s", n)
			continue
		}

		for _, a := range r.Answer {
			switch a := a.(type) {
			case *mdns.PTR:
				// cut the . off the end of the returned record & store it
				domains[strings.TrimSuffix(a.Ptr, ".")] = true
			default:
				log.Debugf("got nonstandard answer %s (from nameserver %s)", a, n)
			}
		}
	}

	return
}

// reverseaddr returns the in-addr.arpa. or ip6.arpa. hostname of the IP
// address addr suitable for rDNS (PTR) record lookup or an error if it fails
// to parse the IP address.
// this helper func was lifted from stdlib -- net/dnsclient.go
func reverseaddr(addr string) (arpa string, err error) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return "", &net.DNSError{Err: "unrecognized address", Name: addr}
	}
	if ip.To4() != nil {
		return strconv.FormatUint(uint64(ip[15]), 10) + "." + strconv.FormatUint(uint64(ip[14]), 10) + "." + strconv.FormatUint(uint64(ip[13]), 10) + "." + strconv.FormatUint(uint64(ip[12]), 10) + ".in-addr.arpa.", nil
	}
	// Must be IPv6
	buf := make([]byte, 0, len(ip)*4+len("ip6.arpa."))
	// Add it, in reverse, to the buffer
	for i := len(ip) - 1; i >= 0; i-- {
		v := ip[i]
		buf = append(buf, hexDigit[v&0xF])
		buf = append(buf, '.')
		buf = append(buf, hexDigit[v>>4])
		buf = append(buf, '.')
	}
	// Append "ip6.arpa." and return (buf already has the final .)
	buf = append(buf, "ip6.arpa."...)
	return string(buf), nil
}
