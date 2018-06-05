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

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"context"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	vchconfig "github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/guest"
	"github.com/vmware/vic/lib/pprof"
	"github.com/vmware/vic/pkg/certificate"
	viclog "github.com/vmware/vic/pkg/log"
	"github.com/vmware/vic/pkg/log/syslog"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
	"github.com/vmware/vic/pkg/vsphere/compute"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

const (
	timeout = time.Duration(2 * time.Second)
)

type serverCertificate struct {
	Key  bytes.Buffer
	Cert bytes.Buffer
}

type vicAdminConfig struct {
	session.Config
	addr       string
	tls        bool
	serverCert *serverCertificate
}

var (
	logFileDir          = "/var/log/vic"
	logFileListPrefixes = []string{
		"docker-personality.log",
		"port-layer.log",
		"vicadmin.log",
		"init.log",
		"kubelet-starter.log",
		"virtual-kubelet.log",
	}

	// VMFiles is the set of files to collect per VM associated with the VCH
	vmFiles = []string{
		"output.log",
		"vmware.log",
		"tether.debug",
	}

	// this struct holds root credentials or vSphere extension private key instead if available
	// if you are exposing log information to a user, create a new session for that user, do not use this one
	// also, 'root' is a pun -- this is both the "root" config, e.g., the base config, and the one w/ root creds
	rootConfig vicAdminConfig

	resources vchconfig.Resources

	vchConfig vchconfig.VirtualContainerHostConfigSpec

	datastore types.ManagedObjectReference
)

type logfile struct {
	URL    url.URL
	VMName string
	Host   *object.HostSystem
}

func Init() {
	// #nosec: Errors unhandled.
	_ = pprof.StartPprof("vicadmin", pprof.VicadminPort)

	defer trace.End(trace.Begin(""))

	// load the vch config
	src, err := extraconfig.GuestInfoSource()
	if err != nil {
		log.Errorf("Unable to load configuration from guestinfo")
		return
	}

	extraconfig.Decode(src, &vchConfig)

	logcfg := viclog.NewLoggingConfig()
	if vchConfig.Diagnostics.DebugLevel > 0 {
		logcfg.Level = log.DebugLevel
		trace.Logger.Level = log.DebugLevel
		syslog.Logger.Level = log.DebugLevel
	}

	if vchConfig.Diagnostics.SysLogConfig != nil {
		logcfg.Syslog = &viclog.SyslogConfig{
			Network:  vchConfig.Diagnostics.SysLogConfig.Network,
			RAddr:    vchConfig.Diagnostics.SysLogConfig.RAddr,
			Priority: syslog.Info | syslog.Daemon,
		}
	}

	viclog.Init(logcfg)
	trace.InitLogger(logcfg)

	// We don't want to run this as root.
	ud := syscall.Getuid()
	gd := syscall.Getgid()
	log.Info(fmt.Sprintf("Current UID/GID = %d/%d", ud, gd))
	// TODO: Enable this after we figure out to NOT break the test suite with it.
	// if ud == 0 {
	// log.Errorf("Error: vicadmin must not run as root.")
	// time.Sleep(60 * time.Second)
	// os.Exit(1)
	// }

	flag.StringVar(&rootConfig.addr, "l", "client.localhost:2378", "Listen address")

	// TODO: This should all be pulled from the config
	flag.StringVar(&rootConfig.DatacenterPath, "dc", "", "Path of the datacenter")
	flag.StringVar(&rootConfig.ClusterPath, "cluster", "", "Path of the cluster")
	flag.StringVar(&rootConfig.PoolPath, "pool", "", "Path of the resource pool")

	if vchConfig.HostCertificate == nil {
		log.Infoln("--no-tls is enabled on the personality")
		rootConfig.serverCert = &serverCertificate{}
		rootConfig.serverCert.Cert, rootConfig.serverCert.Key, err = certificate.CreateSelfSigned(rootConfig.addr, []string{"VMware, Inc."}, 2048)
		if err != nil {
			log.Errorf("--no-tls was specified but we couldn't generate a self-signed cert for vic admin due to error %s so vicadmin will not run", err.Error())
			return
		}
	}

	// FIXME: pull the rest from flags
	flag.Parse()
}

type entryReader interface {
	open() (entry, error)
}

type entry interface {
	io.ReadCloser
	Name() string
	Size() int64
}

type bytesEntry struct {
	io.ReadCloser
	name string
	size int64
}

func (e *bytesEntry) Name() string {
	return e.name
}

func (e *bytesEntry) Size() int64 {
	return e.size
}

func newBytesEntry(name string, b []byte) entry {
	r := bytes.NewReader(b)

	return &bytesEntry{
		ReadCloser: ioutil.NopCloser(r),
		size:       int64(r.Len()),
		name:       name,
	}
}

type versionReader string

func (path versionReader) open() (entry, error) {
	defer trace.End(trace.Begin(string(path)))
	return newBytesEntry(string(path), []byte(version.GetBuild().ShortVersion())), nil
}

type commandReader string

func (path commandReader) open() (entry, error) {
	defer trace.End(trace.Begin(string(path)))

	args := strings.Split(string(path), " ")
	// #nosec: Subprocess launching with variable
	cmd := exec.Command(args[0], args[1:]...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, string(output))
	}

	return newBytesEntry(string(path), output), nil
}

type fileReader string

type fileEntry struct {
	io.ReadCloser
	os.FileInfo
}

func (path fileReader) open() (entry, error) {
	defer trace.End(trace.Begin(string(path)))

	f, err := os.Open(string(path))
	if err != nil {
		return nil, err
	}

	s, err := os.Stat(string(path))
	if err != nil {
		return nil, err
	}

	// Files in /proc always have struct stat.st_size==0, so just read it into memory.
	if s.Size() == 0 && strings.HasPrefix(f.Name(), "/proc/") {
		b, err := ioutil.ReadAll(f)
		// #nosec: Errors unhandled.
		_ = f.Close()
		if err != nil {
			return nil, err
		}

		return newBytesEntry(f.Name(), b), nil
	}

	return &fileEntry{
		ReadCloser: f,
		FileInfo:   s,
	}, nil
}

type urlReader string

func httpEntry(name string, res *http.Response) (entry, error) {
	defer trace.End(trace.Begin(name))

	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}

	if res.ContentLength > 0 {
		return &bytesEntry{
			ReadCloser: res.Body,
			size:       res.ContentLength,
			name:       name,
		}, nil
	}

	// If we don't have Content-Length, read into memory for the tar.Header.Size
	body, err := ioutil.ReadAll(res.Body)
	// #nosec: Errors unhandled.
	_ = res.Body.Close()
	if err != nil {
		return nil, err
	}

	return newBytesEntry(name, body), nil
}

func (path urlReader) open() (entry, error) {
	defer trace.End(trace.Begin(string(path)))
	client := http.Client{
		Timeout: timeout,
	}
	res, err := client.Get(string(path))
	if err != nil {
		return nil, err
	}

	return httpEntry(string(path), res)
}

type datastoreReader struct {
	ds   *object.Datastore
	path string
	ctx  context.Context
}

// listVMPaths returns an array of datastore paths for VMs associated with the
// VCH - this includes containerVMs and the appliance
func listVMPaths(ctx context.Context, s *session.Session) ([]logfile, error) {
	defer trace.End(trace.Begin(""))

	var err error
	var children []*vm.VirtualMachine

	if len(vchConfig.ComputeResources) == 0 {
		return nil, errors.New("compute resources is empty")
	}

	ref := vchConfig.ComputeResources[0]
	rp := compute.NewResourcePool(ctx, s, ref)
	op := trace.NewOperation(ctx, "GetChildren")
	if children, err = rp.GetChildrenVMs(op); err != nil {
		return nil, err
	}

	self, err := guest.GetSelf(ctx, s)
	if err != nil {
		log.Errorf("Unable to get handle to self for log filtering")
	}

	log.Infof("Found %d candidate VMs in resource pool %s for log collection", len(children), ref.String())

	logfiles := []logfile{}
	for _, child := range children {
		path, err := child.VMPathNameAsURL(ctx)

		if err != nil {
			log.Errorf("Unable to get datastore path for child VM %s: %s", child.Reference(), err)
			// we need to get as many logs as possible
			continue
		}

		logname, err := child.ObjectName(ctx)
		if err != nil {
			log.Errorf("Unable to get the vm name for %s: %s", child.Reference(), err)
			continue
		}

		if self != nil && child.Reference().String() == self.Reference().String() {
			// FIXME: until #2630 is addressed, and we confirm this filters secrets from appliance vmware.log as well,
			// we're skipping direct collection of those logs.
			log.Info("Skipping collection for appliance VM (moref match)")
			continue
		}

		// backup check if we were unable to initialize self for some reason
		if self == nil && logname == vchConfig.Name {
			log.Info("Skipping collection for appliance VM (string match)")
			continue
		}

		log.Debugf("Adding VM for log collection: %s", path.String())
		h, err := child.HostSystem(ctx)
		if err != nil {
			log.Warnf("Unable to get host system for VM %s - will use default host for log collection: %s", logname, err)
		}

		log := logfile{
			URL:    path,
			VMName: logname,
			Host:   h,
		}

		logfiles = append(logfiles, log)
	}

	log.Infof("Collecting logs from %d VMs", len(logfiles))
	log.Infof("Found VM paths are : %#v", logfiles)
	return logfiles, nil
}

// addApplianceLogs whitelists the logs to include for the appliance.
// TODO: once we've started encrypting all potentially sensitive data and filtering out guestinfo.ovfEnv
// we can resume collection of vmware.log and drop the appliance specific handling
func addApplianceLogs(ctx context.Context, s *session.Session, readers map[string]entryReader) error {
	self, err := guest.GetSelf(ctx, s)
	if err != nil || self == nil {
		return fmt.Errorf("Unable to collect appliance logs due to unknown self-reference: %s", err)
	}

	self2 := vm.NewVirtualMachineFromVM(ctx, s, self)
	path, err := self2.VMPathNameAsURL(ctx)
	if err != nil {
		return err
	}

	ds, err := s.Finder.Datastore(ctx, path.Host)
	if err != nil {
		return err
	}

	h, err := self2.HostSystem(ctx)
	if err != nil {
		log.Warnf("Unable to get host system for appliance - will use default host for log collection: %s", err)
	} else {
		ctx = ds.HostContext(ctx, h)
	}

	wpath := fmt.Sprintf("appliance/tether.debug")
	rpath := fmt.Sprintf("%s/%s", path.Path, "tether.debug")
	log.Infof("Processed File read Path : %s", rpath)
	log.Infof("Processed File write Path : %s", wpath)
	readers[wpath] = datastoreReader{
		ds:   ds,
		path: rpath,
		ctx:  ctx,
	}

	return nil
}

// find datastore logs for the appliance itself and all containers
func findDatastoreLogs(c *session.Session) (map[string]entryReader, error) {
	defer trace.End(trace.Begin(""))

	// Create an empty reader as opposed to a nil reader...
	readers := map[string]entryReader{}
	ctx := context.Background()

	logfiles, err := listVMPaths(ctx, c)
	if err != nil {
		detail := fmt.Sprintf("unable to perform datastore log collection due to failure looking up paths: %s", err)
		log.Error(detail)
		return nil, errors.New(detail)
	}

	err = addApplianceLogs(ctx, c, readers)
	if err != nil {
		log.Errorf("Issue collecting appliance logs: %s", err)
	}

	for _, logfile := range logfiles {
		log.Debugf("Assembling datastore readers for %s", logfile.URL.String())
		// obtain datastore object
		ds, err := c.Finder.Datastore(ctx, logfile.URL.Host)
		if err != nil {
			log.Errorf("Failed to acquire reference to datastore %s: %s", logfile.URL.Host, err)
			continue
		}

		hCtx := ctx
		if logfile.Host != nil {
			hCtx = ds.HostContext(ctx, logfile.Host)
		}

		// generate the full paths to collect
		for _, file := range vmFiles {
			wpath := fmt.Sprintf("%s/%s", logfile.VMName, file)
			rpath := fmt.Sprintf("%s/%s", logfile.URL.Path, file)
			log.Infof("Processed File read Path : %s", rpath)
			log.Infof("Processed File write Path : %s", wpath)
			readers[wpath] = datastoreReader{
				ds:   ds,
				path: rpath,
				ctx:  hCtx,
			}

			log.Debugf("Added log file for collection: %s", logfile.URL.String())
		}
	}

	return readers, nil
}

func (r datastoreReader) open() (entry, error) {
	defer trace.End(trace.Begin(r.path))

	u, ticket, err := r.ds.ServiceTicket(r.ctx, r.path, "GET")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if ticket != nil {
		req.AddCookie(ticket)
	}

	res, err := r.ds.Client().Do(req)
	if err != nil {
		return nil, err
	}

	return httpEntry(r.path, res)
}

// stripCredentials removes user credentials from "in"
func stripCredentials(in *session.Session) error {
	serviceURL, err := soap.ParseURL(rootConfig.Service)
	if err != nil {
		log.Errorf("Error parsing service URL from config: %s", err)
		return err
	}
	serviceURL.User = nil
	newclient, err := govmomi.NewClient(context.Background(), serviceURL, true)
	if err != nil {
		log.Errorf("Error creating new govmomi client without credentials but with auth cookie: %s", err.Error())
		return err
	}
	newclient.Jar = in.Client.Jar
	in.Client = newclient
	return nil
}

func vSphereSessionGet(sessconfig *session.Config) (*session.Session, error) {
	s := session.NewSession(sessconfig)
	s.UserAgent = version.UserAgent("vic-admin")

	ctx := context.Background()
	_, err := s.Connect(ctx)
	if err != nil {
		log.Warnf("Unable to connect: %s", err)
		return nil, err
	}

	_, err = s.Populate(ctx)
	if err != nil {
		// not a critical error for vicadmin
		log.Warnf("Unable to populate session: %s", err)
	}
	usersession, err := s.SessionManager.UserSession(ctx)
	if err != nil {
		log.Errorf("Got %s while creating user session", err)
		return nil, err
	}
	if usersession == nil {
		return nil, fmt.Errorf("vSphere session is no longer valid")
	}

	log.Infof("Got session from vSphere with key: %s username: %s", usersession.Key, usersession.UserName)

	err = stripCredentials(s)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *server) getSessionFromRequest(ctx context.Context, r *http.Request) (*session.Session, error) {
	sessionData, err := s.uss.cookies.Get(r, sessionCookieKey)
	if err != nil {
		return nil, err
	}
	var d interface{}
	var ok bool
	if d, ok = sessionData.Values[sessionKey]; !ok {
		return nil, fmt.Errorf("User-provided cookie did not contain a session ID -- it is corrupt or tampered")
	}
	c, err := s.uss.VSphere(ctx, d.(string))
	return c, err
}

type flushWriter struct {
	f http.Flusher
	w io.Writer
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)

	fw.f.Flush()

	return n, err
}

func main() {
	Init()

	if version.Show() {
		fmt.Fprintf(os.Stdout, "%s\n", version.String())
		return
	}

	// FIXME: these should just be consumed directly inside Session
	rootConfig.Service = vchConfig.Target
	rootConfig.User = url.UserPassword(vchConfig.Username, vchConfig.Token)
	rootConfig.Thumbprint = vchConfig.TargetThumbprint
	rootConfig.DatastorePath = vchConfig.Storage.ImageStores[0].Host

	if vchConfig.Diagnostics.DebugLevel > 0 {
		log.SetLevel(log.DebugLevel)
		log.Info("Setting debug logging")
	}

	if vchConfig.Diagnostics.DebugLevel > 2 {
		rootConfig.addr = "0.0.0.0:2378"
		log.Warn("Listening on all networks because of debug level")
	}
	s := &server{
		addr: rootConfig.addr,
	}

	err := s.listen()

	if err != nil {
		log.Fatal(err)
	}

	log.Infof("listening on %s", s.addr)
	signals := []syscall.Signal{
		syscall.SIGTERM,
		syscall.SIGINT,
	}

	sigchan := make(chan os.Signal, 1)
	for _, signum := range signals {
		signal.Notify(sigchan, signum)
	}

	go func() {
		signal := <-sigchan
		log.Infof("received %s", signal)
		s.stop()
	}()

	s.serve()
}
