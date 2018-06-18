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

const (
	// fileWriteFlags is a collection of flags configuring our writes for general tar behavior
	//
	// O_CREATE = Create file if it does not exist
	// O_TRUNC = truncate file to 0 length if it does exist(overwrite the file)
	// O_WRONLY = We use this since we do not intend to read, we only need to write.
	fileWriteFlags = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
)

// InvokeUnpack will unpack the given tarstream(if it is a tar stream) on the local filesystem based on the specified root
// combined with any rebase from the path spec
//
// the pathSpec will include the following elements
// - include : any tar entry that has a path below(after stripping) the include path will be written
// - strip : The strip string will indicate the
// - exlude : marks paths that are to be excluded from the write
// - rebase : marks the the write path that will be tacked onto (appended or prepended? TODO improve this comment) the "root". e.g /tmp/unpack + /my/target/path = /tmp/unpack/my/target/path
// N.B. tarStream MUST BE TERMINATED WITH EOF or this function will hang forever!
func InvokeUnpack(op trace.Operation, tarStream io.Reader, filter *FilterSpec, root string) error {
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

// Unpack hooks into a binary present in the appliance vm called unpack in order to execute InvokeUnpack inside of a chroot. This method works identically to InvokeUnpack, except that it will not function in areas where the binary is not present at /bin/unpack
func Unpack(op trace.Operation, tarStream io.Reader, filter *FilterSpec, root string) error {

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

	encodedFilter, err := EncodeFilterSpec(op, filter)
	if err != nil {
		op.Error(err)
		return err
	}

	// Prepare to launch the binary, which will create a chroot at root and then invoke InvokeUnpack
	// #nosec: Subprocess launching with variable. -- neither variable is user input & both are bounded inputs so this is fine
	cmd := exec.Command("/bin/unpack", op.ID(), root, *encodedFilter)

	//stdin
	stdin, err := cmd.StdinPipe()

	if err != nil {
		op.Error(err)
		return err
	}

	if stdin == nil {
		err = errors.New("stdin was nil")
		op.Error(err)
		return err
	}
	done := make(chan error)
	go func() {
		// copy the tarStream to the binary via stdin; the binary will stream it to InvokeUnpack unchanged
		defer stdin.Close()
		if _, err := io.Copy(stdin, tarStream); err != nil {
			op.Errorf("Error copying tarStream: %s", err.Error())
		}
		done <- err
	}()

	out, err := cmd.CombinedOutput()
	if len(out) == 0 {
		op.Debug("No output from command")
	} else {
		// output should just be trace messages
		op.Debugf("%s", string(out))
	}

	if err != nil {
		stdin.Close()
		op.Errorf("Command returned error %s", err.Error())
		return err
	}

	// This error gets logged by the goroutine if it is non-nil.
	// This receive is just functioning as a wait
	err = <-done
	return err
}
