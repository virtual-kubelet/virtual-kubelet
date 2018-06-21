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

// +build !windows

package archive

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"errors"

	"github.com/vmware/vic/pkg/trace"
)

type binaryPath string

const (
	// fileWriteFlags is a collection of flags configuring our writes for general tar behavior
	//
	// O_CREATE = Create file if it does not exist
	// O_TRUNC = truncate file to 0 length if it does exist(overwrite the file)
	// O_WRONLY = We use this since we do not intend to read, we only need to write.
	fileWriteFlags = os.O_CREATE | os.O_TRUNC | os.O_WRONLY

	// Location of the unpack binary inside of containers
	containerBinaryPath binaryPath = "/.tether/unpack"

	// Location of the unpack binary inside the endpoint VM
	applianceBinaryPath binaryPath = "/bin/unpack"
)

// UnpackNoChroot will unpack the given tarstream(if it is a tar stream) on the local filesystem based on the specified root
// combined with any rebase from the path spec
//
// the pathSpec will include the following elements
// - include : any tar entry that has a path below(after stripping) the include path will be written
// - strip : The strip string will indicate the
// - exlude : marks paths that are to be excluded from the write
// - rebase : marks the the write path that will be tacked onto (appended or prepended? TODO improve this comment) the "root". e.g /tmp/unpack + /my/target/path = /tmp/unpack/my/target/path
// N.B. tarStream MUST BE TERMINATED WITH EOF or this function will hang forever!
func UnpackNoChroot(op trace.Operation, tarStream io.Reader, filter *FilterSpec, root string) error {
	op.Debugf("unpacking archive to root: %s, filter: %+v", root, filter)

	// Online datasource is sending a tar reader instead of an io reader.
	// Type check here to see if we actually need to create a tar reader.
	var tr *tar.Reader
	if trCheck, ok := tarStream.(*tar.Reader); ok {
		tr = trCheck
	} else {
		tr = tar.NewReader(tarStream)
	}

	fi, err := os.Stat(root)
	if err != nil {
		// the target unpack path does not exist. We should not get here.
		op.Errorf("tar unpack target does not exist: %s", root)
		return err
	}

	if !fi.IsDir() {
		err := fmt.Errorf("unpack root target is not a directory: %s", root)
		op.Error(err)
		return err
	}

	op.Debugf("using FilterSpec : (%#v)", *filter)
	// process the tarball onto the filesystem
	for {
		header, err := tr.Next()
		if err == io.EOF {
			// This indicates the end of the archive
			break
		}

		if err != nil {
			op.Errorf("Error reading tar header: %s", err)
			return err
		}

		op.Debugf("processing tar header: asset(%s), size(%d)", header.Name, header.Size)
		// skip excluded elements unless explicitly included
		if filter.Excludes(op, header.Name) {
			continue
		}

		// fix up path
		stripped := strings.TrimPrefix(header.Name, filter.StripPath)
		rebased := filepath.Join(filter.RebasePath, stripped)
		absPath := filepath.Join(root, rebased)

		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(absPath, header.FileInfo().Mode())
			if err != nil {
				op.Errorf("Failed to create directory%s: %s", absPath, err)
				return err
			}
		case tar.TypeSymlink:
			err := os.Symlink(header.Linkname, absPath)
			if err != nil {
				op.Errorf("Failed to create symlink %s->%s: %s", absPath, header.Linkname, err)
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(absPath, fileWriteFlags, header.FileInfo().Mode())
			if err != nil {
				op.Errorf("Failed to open file %s: %s", absPath, err)
				return err
			}
			_, err = io.Copy(f, tr)
			// TODO: add ctx.Done cancellation
			f.Close()
			if err != nil {
				return err
			}
		default:
			// TODO: add support for special file types - otherwise we will do absurd things such as read infinitely from /dev/random
		}
		op.Debugf("Finished writing to: %s", absPath)
	}
	return nil
}

// OfflineUnpack wraps Unpack for usage in contexts without a childReaper, namely when copying to an offline container with docker cp
func OfflineUnpack(op trace.Operation, tarStream io.Reader, filter *FilterSpec, root string) error {

	var cmd *exec.Cmd
	var err error
	if cmd, err = unpack(op, tarStream, filter, root, applianceBinaryPath); err != nil {
		return err
	}

	if err = cmd.Wait(); err != nil {
		return err
	}

	return nil
}

// OnlineUnpack will extract a tar stream tarStream to folder root inside of a running container
func OnlineUnpack(op trace.Operation, tarStream io.Reader, filter *FilterSpec, root string) (*exec.Cmd, error) {
	return unpack(op, tarStream, filter, root, containerBinaryPath)
}

func streamCopy(op trace.Operation, stdin io.WriteCloser, tarStream io.Reader) error {
	// if we're passed a stream that doesn't cast to a tar.Reader copy the tarStream to the binary via stdin; the binary will stream it to InvokeUnpack unchanged
	var err error
	tr, ok := tarStream.(*tar.Reader)
	if !ok {
		defer stdin.Close()
		if _, err := io.Copy(stdin, tarStream); err != nil {
			op.Errorf("Error copying tarStream: %s", err.Error())
			return err
		}
		return nil
	}

	tw := tar.NewWriter(stdin)
	defer tw.Close()
	var th *tar.Header
	for {
		th, err = tr.Next()
		if err == io.EOF {
			tw.Close()
			return nil
		}
		if err != nil {
			op.Errorf("error reading tar header %s", err)
			return err
		}
		op.Debugf("processing tar header: asset(%s), size(%d)", th.Name, th.Size)
		err = tw.WriteHeader(th)
		if err != nil {
			op.Errorf("error writing tar header %s", err)
			return err
		}
		var k int64
		k, err = io.Copy(tw, tr)
		op.Debugf("wrote %d bytes", k)
		if err != nil {
			op.Errorf("error writing file body bytes to stdin %s", err)
			return err
		}
	}
}

func DockerUnpack(op trace.Operation, root string, tarStream io.Reader) (int64, error) {
	fi, err := os.Stat(root)
	if err != nil {
		// the target unpack path does not exist. We should not get here.
		return 0, err
	}

	if !fi.IsDir() {
		return 0, fmt.Errorf("unpack root target is not a directory: %s", root)
	}

	// #nosec: 193 applianceBinaryPath is a constant, not a variable
	cmd := exec.Command(string(applianceBinaryPath), root)

	stdin, err := cmd.StdinPipe()

	if err != nil {
		return 0, err
	}

	if stdin == nil {
		err = errors.New("stdin was nil")
		return 0, err
	}

	if err = cmd.Start(); err != nil {
		return 0, err
	}

	bytesWritten := make(chan int64, 1)
	go func() {
		defer stdin.Close()
		var n int64
		if n, err = io.Copy(stdin, tarStream); err != nil {
			op.Errorf("Error copying tarStream: %s", err.Error())
		}
		bytesWritten <- n
	}()

	if err = cmd.Wait(); err != nil {
		return 0, err
	}

	return <-bytesWritten, nil
}

// Unpack runs the binary compiled in cmd/unpack.go which creates a chroot at `root` and passes `op`, `tarStream`, and `filter` to InvokeUnpack for extraction of the tar on the filesystem. `binPath` should be either ApplianceBinaryPath or ContainerBinaryPath. Unpack returns a `Cmd` to allow use in conjunction with the tether's `LaunchUtility`, so it is necessary to call `cmd.Wait` after `Unpack` exits e.g. OfflineUnpack, if not being used in conjunction with LaunchUtility and the childReaper.
func unpack(op trace.Operation, tarStream io.Reader, filter *FilterSpec, root string, binPath binaryPath) (*exec.Cmd, error) {

	fi, err := os.Stat(root)
	if err != nil {
		// the target unpack path does not exist. We should not get here.
		op.Errorf("tar unpack target does not exist: %s", root)
		return nil, err
	}

	if !fi.IsDir() {
		err := fmt.Errorf("unpack root target is not a directory: %s", root)
		op.Error(err)
		return nil, err
	}

	encodedFilter, err := EncodeFilterSpec(op, filter)
	if err != nil {
		op.Error(err)
		return nil, err
	}

	// Prepare to launch the binary, which will create a chroot at root and then invoke InvokeUnpack
	// #nosec: Subprocess launching with variable. -- neither variable is user input & both are bounded inputs so this is fine
	// "/bin/unpack" on appliance
	// "/.tether/unpack" inside container
	cmd := exec.Command(string(binPath), root, *encodedFilter)

	stdin, err := cmd.StdinPipe()

	if err != nil {
		op.Error(err)
		return nil, err
	}

	if stdin == nil {
		err = errors.New("stdin was nil")
		op.Error(err)
		return nil, err
	}

	go streamCopy(op, stdin, tarStream)

	return cmd, cmd.Start()

}
