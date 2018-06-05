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
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"context"

	log "github.com/Sirupsen/logrus"
	"github.com/hpcloud/tail"

	"path/filepath"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/lib/pprof"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/session"
)

const (
	nBytes    = 1024
	tailLines = 8
	uint32max = (1 << 32) - 1

	// how many lines of log data to collect
	logLines = 5000
	// how many lines to request per call
	lines = 500
)

type dlogReader struct {
	c    *session.Session
	name string
	host *object.HostSystem
}

func (r dlogReader) open() (entry, error) {
	defer trace.End(trace.Begin(r.name))

	name := r.name
	if r.host != nil {
		name = fmt.Sprintf("%s-%s", path.Base(r.host.InventoryPath), r.name)
	}

	m := object.NewDiagnosticManager(r.c.Vim25())
	ctx := context.Background()

	// Currently we collect the tail of diagnostic log files to avoid
	// reading the entire file into memory or writing local disk.

	// get LineEnd without any LineText
	h, err := m.BrowseLog(ctx, r.host, r.name, math.MaxInt32, 0)
	if err != nil {
		return nil, err
	}

	end := h.LineEnd
	start := end - logLines

	var buf bytes.Buffer
	for start < end {
		h, err = m.BrowseLog(ctx, r.host, r.name, start, lines)
		if err != nil {
			return nil, err
		}

		for _, line := range h.LineText {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}

		start += lines
	}

	return newBytesEntry(name+".log", buf.Bytes()), nil
}

// logFiles has a potential race condition since logrotation can rotate files this moment of time.
// however, the likely hood of this race is so low that it doesn't worth investing the time to do
// cross process synchronization.
func logFiles() []string {
	defer trace.End(trace.Begin(""))
	files, err := ioutil.ReadDir(logFileDir)
	if err != nil {
		log.Errorf("Failed to get a list of log files: %s", err)
		return nil
	}

	names := []string{}
	for _, fileInfo := range files {
		if fileInfo.IsDir() {
			continue
		}
		fname := fileInfo.Name()
		log.Debugf("Found potential file for export: %s", fname)

		for _, f := range logFileListPrefixes {
			if strings.HasPrefix(fname, f) {
				fp := filepath.Join(logFileDir, fname)
				log.Debugf("Adding file for export: %s", fp)
				names = append(names, fp)
				break
			}
		}
	}

	return names
}

// logFile writes the contents of file f and any of it's rotated files to the http writer.
// includeF is used for tailing, in which case we want to write rotated files but not f itself
func writeLogFiles(w http.ResponseWriter, r *http.Request, f string, includeF bool) {
	defer trace.End(trace.Begin(""))
	files, err := ioutil.ReadDir(logFileDir)
	if err != nil {
		log.Errorf("Failed to get a list of log files: %s", err)
		return
	}

	// find all rotated files, but not f itself
	names := []string{}
	for _, fileInfo := range files {
		fname := fileInfo.Name()
		if fileInfo.IsDir() || (fname == f) {
			continue
		}
		log.Debugf("Found potential file for export: %s", fname)

		if strings.HasPrefix(fname, f) {
			fp := filepath.Join(logFileDir, fname)
			names = append(names, fp)
		}
	}
	// sort file names to preserve time rotation order
	sort.Strings(names)

	// find f last and append it to names
	for _, fileInfo := range files {
		fname := fileInfo.Name()
		if fileInfo.IsDir() {
			continue
		}
		if fname == f {
			log.Debugf("Found potential file for export: %s", fname)
			fp := filepath.Join(logFileDir, fname)
			names = append(names, fp)
		}
	}

	if len(names) == 0 {
		http.NotFound(w, r)
	}

	// write file contents to w
	for _, fileName := range names {
		file, err := os.Open(fileName)
		log.Debugf("Writing contents of: %s", fileName)
		if err != nil {
			log.Errorf("error opening file %s: %s", fileName, err.Error())
			continue
		}
		// using interface type here so we can reassign r as a gzip reader for rotated logs
		var r io.ReadCloser = file
		if strings.HasSuffix(fileName, "gz") {
			r, err = gzip.NewReader(file)
			if err != nil {
				log.Errorf("error opening gzipped file %s: %s", fileName, err.Error())
				continue
			}
		}
		_, err = io.Copy(w, r)
		if err != nil {
			log.Errorf("error writing contents of %s: %s", fileName, err.Error())
			continue
		}
		r.Close()
		file.Close()
	}

}

func configureReaders() map[string]entryReader {
	defer trace.End(trace.Begin(""))

	pprofPaths := map[string]string{
		// verbose
		"verbose": "/debug/pprof/goroutine?debug=2",
		// concise
		"concise": "/debug/pprof/goroutine?debug=1",
		"block":   "/debug/pprof/block?debug=1",
		"heap":    "/debug/pprof/heap?debug=1",
		"profile": "/debug/pprof/profile",
	}

	pprofSources := map[string]string{
		"docker":    pprof.GetPprofEndpoint(pprof.DockerPort).String(),
		"portlayer": pprof.GetPprofEndpoint(pprof.PortlayerPort).String(),
		"vicadm":    pprof.GetPprofEndpoint(pprof.VicadminPort).String(),
		"vic-init":  pprof.GetPprofEndpoint(pprof.VCHInitPort).String(),
	}

	readers := map[string]entryReader{
		"proc-mounts": fileReader("/proc/mounts"),
		"uptime":      commandReader("uptime"),
		"df":          commandReader("df"),
		"free":        commandReader("free"),
		"netstat":     commandReader("netstat -ant"),
		"iptables":    commandReader("sudo iptables --list"),
		"ip-link":     commandReader("ip link"),
		"ip-addr":     commandReader("ip addr"),
		"ip-route":    commandReader("ip route"),
		"lsmod":       commandReader("lsmod"),
		// TODO: ls without shelling out
		"disk-by-path":  commandReader("ls -l /dev/disk/by-path"),
		"disk-by-label": commandReader("ls -l /dev/disk/by-label"),
		"disk-by-uuid":  commandReader("ls -l /dev/disk/by-uuid"),
		"lsblk":         commandReader("lsblk -S"),
		// To check we are not leaking any fds
		"proc-self-fd": commandReader("ls -l /proc/self/fd"),
		"ps":           commandReader("ps -ef"),
		"meminfo":      fileReader("/proc/meminfo"),
		"journalctl":   commandReader("/bin/journalctl --no-pager"),
		"dmesg":        commandReader("/bin/journalctl --dmesg --no-pager"),
		"sys-block":    commandReader("ls -l /sys/block/"),
		// To check version
		"VERSION": versionReader("version"),
	}

	// add the pprof collection
	for sname, source := range pprofSources {
		for pname, paths := range pprofPaths {
			rname := fmt.Sprintf("%s/%s", sname, pname)
			readers[rname] = urlReader(source + paths)
		}
	}

	for _, path := range logFiles() {
		// Strip off leading '/'
		readers[path[1:]] = fileReader(path)
	}

	return readers
}

func findDiagnosticLogs(c *session.Session) (map[string]entryReader, error) {
	defer trace.End(trace.Begin(""))

	// When connected to VC, we collect vpxd.log and hostd.log for all cluster hosts attached to the datastore.
	// When connected to ESX, we just collect hostd.log.
	const (
		vpxdKey  = "vpxd:vpxd.log"
		hostdKey = "hostd"
	)

	logs := map[string]entryReader{}
	var err error

	if c.IsVC() {
		logs[vpxdKey] = dlogReader{c, vpxdKey, nil}

		var hosts []*object.HostSystem
		if c.Cluster == nil && c.Host != nil {
			hosts = []*object.HostSystem{c.Host}
		} else {
			hosts, err = c.Datastore.AttachedClusterHosts(context.TODO(), c.Cluster)
			if err != nil {
				return nil, err
			}
		}

		for _, host := range hosts {
			lname := fmt.Sprintf("%s/%s", hostdKey, host)
			logs[lname] = dlogReader{c, hostdKey, host}
		}
	} else {
		logs[hostdKey] = dlogReader{c, hostdKey, nil}
	}

	return logs, nil
}

func tarEntries(readers map[string]entryReader, out io.Writer) error {
	defer trace.End(trace.Begin(""))

	r, w := io.Pipe()
	t := tar.NewWriter(w)

	wg := new(sync.WaitGroup)
	wg.Add(1)

	// stream tar to out
	go func() {
		_, err := io.Copy(out, r)
		if err != nil {
			log.Errorf("error copying tar: %s", err)
		}
		wg.Done()
	}()

	for name, r := range readers {
		log.Infof("Collecting log with reader %s(%#v)", name, r)

		e, err := r.open()
		if err != nil {
			log.Warningf("error reading %s(%s): %s\n", name, r, err)
			continue
		}
		var sz int64
		if e != nil {
			sz = e.Size()
		}

		header := tar.Header{
			Name:    name,
			Size:    sz,
			Mode:    0640,
			ModTime: time.Now(),
		}

		err = t.WriteHeader(&header)
		if err != nil {
			log.Errorf("Failed to write header for %s: %s", header.Name, err)
			continue
		}

		log.Infof("%s has size %d", header.Name, header.Size)

		// be explicit about the number of bytes to copy as the log files will likely
		// be written to during this exercise
		if e != nil {
			_, err = io.CopyN(t, e, sz)
			// #nosec: Errors unhandled.
			_ = e.Close()
		}
		if err != nil {
			log.Errorf("Failed to write content for %s: %s", header.Name, err)
			continue
		}
	}

	// #nosec: Errors unhandled.
	_ = t.Flush()
	// #nosec: Errors unhandled.
	_ = w.Close()
	wg.Wait()
	// #nosec: Errors unhandled.
	_ = r.Close()

	return nil
}

func zipEntries(readers map[string]entryReader, out *zip.Writer) error {
	defer trace.End(trace.Begin(""))
	defer out.Close()
	defer out.Flush()

	for name, r := range readers {
		log.Infof("Collecting log with reader %s(%#v)", name, r)

		e, err := r.open()
		if err != nil {
			log.Warningf("error reading %s(%s): %s\n", name, r, err)
		}
		var sz int64
		if e != nil {
			sz = e.Size()
		}
		header := &zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		}

		header.SetModTime(time.Now())
		header.SetMode(0644)
		if sz > uint32max {
			header.UncompressedSize = uint32max
		} else {
			header.UncompressedSize = uint32(sz)
		}

		w, err := out.CreateHeader(header)

		if err != nil {
			log.Errorf("Failed to create Zip writer for %s: %s", header.Name, err)
			continue
		}

		log.Infof("%s has size %d", header.Name, sz)

		// be explicit about the number of bytes to copy as the log files will likely
		// be written to during this exercise
		if e != nil {
			_, err = io.CopyN(w, e, sz)
			// #nosec: Errors unhandled.
			_ = e.Close()
		}
		if err != nil {
			log.Errorf("Failed to write content for %s: %s", header.Name, err)
			continue
		}
		log.Infof("Wrote %d bytes to %s", sz, header.Name)
	}
	return nil
}
func tailFile(wr io.Writer, file string, done *chan bool) error {
	defer trace.End(trace.Begin(file))

	// By default, seek to EOF (if file doesn't exist)
	spos := tail.SeekInfo{
		Offset: 0,
		Whence: 2,
	}

	// If the file exists, we want to go back tailLines lines
	// and pass that new offset into the TailFile() constructor
	// Per @fdawg4l, use bytes.LastIndex() and a 1k buffer to reduce
	// seeks/reads
	f, err := os.Open(file)
	if err == nil {
		spos = tail.SeekInfo{
			Offset: findSeekPos(f),
			Whence: 0,
		}
	}

	tcfg := tail.Config{
		Location:  &spos,
		ReOpen:    true,
		MustExist: false,
		Follow:    true,
	}

	t, err := tail.TailFile(file, tcfg)
	if err != nil {
		return err
	}

	// We KNOW there's a data race here.
	// But it doesn't break anything, so we just trap it.
	defer func() {
		t.Stop()
		_ = recover()
	}()
	for true {
		select {
		case l := <-t.Lines:
			if l.Err != nil {
				return l.Err
			}
			fmt.Fprint(wr, l.Text, "\n")
		case _ = <-*done:
			return nil
		}
	}
	return nil
}

// Find the offset we want to start tailing from.
// This should either be beginning-of-file or tailLines
// newlines from the EOF.
func findSeekPos(f *os.File) int64 {
	defer trace.End(trace.Begin(""))
	nlines := tailLines
	readPos, err := f.Seek(0, 2)
	// If for some reason we can't seek, we will just start tailing from beginning-of-file
	if err != nil {
		return int64(0)
	}

	// Buffer so we can seek nBytes (default: 1k) at a time
	buf := make([]byte, nBytes)

	for readPos > 0 {
		// Go back nBytes from the last readPos we've seen (stopping at beginning-of-file)
		// and read the next nBytes
		readPos -= int64(len(buf))
		if readPos < 0 {
			// We don't want to overlap our read with previous reads...
			buf = buf[:(int(readPos) + nBytes)]
			readPos = 0
		}
		bufend, err := f.ReadAt(buf, readPos)

		// It's OK to get io.EOF here.  Anything else is bad.
		if err != nil && err != io.EOF {
			log.Errorf("Error reading from file %s: %s", f.Name(), err)
			return 0
		}

		// Start from the end of the buffer and start looking for newlines
		for bufend > 0 {
			bufend = bytes.LastIndexByte(buf[:bufend], '\n')
			if bufend < 0 {
				break
			}
			nlines--
			if nlines < 0 {
				return readPos + int64(bufend) + 1
			}
		}
	}
	return 0
}
