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

package nfs

import (
	"io"
	"net/url"
	"os"

	nfsClient "github.com/vmware/go-nfs-client/nfs"
	"github.com/vmware/go-nfs-client/nfs/rpc"

	"github.com/vmware/vic/pkg/trace"
)

// MountServer is an interface used to communicate with network attached storage.
type MountServer interface {
	// Mount executes the mount program on the Target.
	Mount(op trace.Operation) (Target, error)

	// Unmount terminates the Mount on the Target.
	Unmount(op trace.Operation) error

	URL() (*url.URL, error)
}

// Target is the filesystem interface for performing actions against attached storage.
type Target interface {
	// Open opens a file on the Target in RD_ONLY
	Open(path string) (io.ReadCloser, error)

	// OpenFile opens a file on the Target with the given mode
	OpenFile(path string, perm os.FileMode) (io.ReadWriteCloser, error)

	// Mkdir creates a directory at the given path
	Mkdir(path string, perm os.FileMode) ([]byte, error)

	// RemoveAll deletes Directory recursively
	RemoveAll(Path string) error

	// ReadDir reads the dirents in the given directory
	ReadDir(path string) ([]os.FileInfo, error)

	// Lookup reads os.FileInfo for the given path
	Lookup(path string) (os.FileInfo, []byte, error)
}

// NfsMount is used to wrap a MountServer to do the Mount()/Unmount() and Close()
type NfsMount struct {
	// Hostname is the name to authenticate with to the target as
	Hostname string

	// UID and GID are the user id and group id to authenticate with the target
	UID, GID uint32

	// The URL (host + path) of the NFS server and target path
	TargetURL *url.URL

	s *nfsClient.Mount
}

func NewMount(t *url.URL, hostname string, uid, gid uint32) *NfsMount {
	return &NfsMount{
		Hostname:  hostname,
		UID:       uid,
		GID:       gid,
		TargetURL: t,
	}
}

func (m *NfsMount) Mount(op trace.Operation) (Target, error) {
	op.Debugf("Mounting %s", m.TargetURL.String())
	s, err := nfsClient.DialMount(m.TargetURL.Host)
	if err != nil {
		return nil, err
	}
	m.s = s

	defer func() {
		if err != nil {
			// #nosec: Errors unhandled.
			m.s.Close()
		}
	}()

	auth := rpc.NewAuthUnix(m.Hostname, m.UID, m.GID)
	mnt, err := s.Mount(m.TargetURL.Path, auth.Auth())
	if err != nil {
		op.Errorf("unable to mount volume: %v", err)
		return nil, err
	}

	op.Infof("Mounted %s", m.TargetURL.String())
	return &target{mnt}, nil
}

func (m *NfsMount) Unmount(op trace.Operation) error {
	op.Debugf("Unmounting %s", m.TargetURL.String())
	if err := m.s.Unmount(); err != nil {
		return err
	}

	if err := m.s.Close(); err != nil {
		return err
	}

	op.Debugf("Unmounted %s", m.TargetURL.String())
	m.s = nil
	return nil
}

func (m *NfsMount) URL() (*url.URL, error) {
	return m.TargetURL, nil
}

// wrap ReadDir to return a slice of os.FileInfo
type target struct {
	*nfsClient.Target
}

func (t *target) ReadDir(path string) ([]os.FileInfo, error) {
	entries, err := t.ReadDirPlus(path)
	if err != nil {
		return nil, err
	}

	var e []os.FileInfo
	for i := 0; i < len(entries); i++ {

		// filter out . and ..
		name := entries[i].Name()
		if name == "." || name == ".." {
			continue
		}

		e = append(e, os.FileInfo(entries[i]))
	}

	return e, nil
}
