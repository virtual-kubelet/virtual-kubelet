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

// NOTE: This file is borrowed directly from docker in order to use unexported
// functions that it provides.
// https://github.com/moby/moby/blob/master/pkg/archive/archive_unix.go

// +build !windows

package archive

import (
	"archive/tar"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/docker/docker/pkg/system"
)

// fixVolumePathPrefix does platform specific processing to ensure that if
// the path being passed in is not in a volume path format, convert it to one.
func fixVolumePathPrefix(srcPath string) string {
	return srcPath
}

// getWalkRoot calculates the root path when performing a TarWithOptions.
// We use a separate function as this is platform specific. On Linux, we
// can't use filepath.Join(srcPath,include) because this will clean away
// a trailing "." or "/" which may be important.
func getWalkRoot(srcPath string, include string) string {
	return srcPath + string(filepath.Separator) + include
}

// CanonicalTarNameForPath returns platform-specific filepath
// to canonical posix-style path for tar archival. p is relative
// path.
func CanonicalTarNameForPath(p string) (string, error) {
	return p, nil // already unix-style
}

// chmodTarEntry is used to adjust the file permissions used in tar header based
// on the platform the archival is done.

func chmodTarEntry(perm os.FileMode) os.FileMode {
	return perm // noop for unix as golang APIs provide perm bits correctly
}

func getFileUIDGID(stat interface{}) (int, int, error) {
	s, ok := stat.(*syscall.Stat_t)

	if !ok {
		return -1, -1, errors.New("cannot convert stat value to syscall.Stat_t")
	}
	return int(s.Uid), int(s.Gid), nil
}

func major(device uint64) uint64 {
	return (device >> 8) & 0xfff
}

func minor(device uint64) uint64 {
	return (device & 0xff) | ((device >> 12) & 0xfff00)
}

// handleTarTypeBlockCharFifo is an OS-specific helper function used by
// createTarFile to handle the following types of header: Block; Char; Fifo
func handleTarTypeBlockCharFifo(hdr *tar.Header, path string) error {

	mode := uint32(hdr.Mode & 07777)
	switch hdr.Typeflag {
	case tar.TypeBlock:
		mode |= syscall.S_IFBLK
	case tar.TypeChar:
		mode |= syscall.S_IFCHR
	case tar.TypeFifo:
		mode |= syscall.S_IFIFO
	}

	if err := system.Mknod(path, mode, int(system.Mkdev(hdr.Devmajor, hdr.Devminor))); err != nil {
		return err
	}
	return nil
}

func handleLChmod(hdr *tar.Header, path string, hdrInfo os.FileInfo) error {
	if hdr.Typeflag == tar.TypeLink {
		if fi, err := os.Lstat(hdr.Linkname); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			if err := os.Chmod(path, hdrInfo.Mode()); err != nil {
				return err
			}
		}
	} else if hdr.Typeflag != tar.TypeSymlink {
		if err := os.Chmod(path, hdrInfo.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func setHeaderForSpecialDevice(hdr *tar.Header, stat interface{}) (inode uint64, err error) {
	s, ok := stat.(*syscall.Stat_t)

	if !ok {
		err = errors.New("cannot convert stat value to syscall.Stat_t")
		return
	}

	inode = uint64(s.Ino)

	// Currently go does not fill in the major/minors
	if s.Mode&syscall.S_IFBLK != 0 ||
		s.Mode&syscall.S_IFCHR != 0 {
		hdr.Devmajor = int64(major(uint64(s.Rdev)))
		hdr.Devminor = int64(minor(uint64(s.Rdev)))
	}

	return
}

func hasHardlinks(fi os.FileInfo) bool {
	return fi.Sys().(*syscall.Stat_t).Nlink > 1
}
