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

package archive

import (
	"archive/tar"
	"io"

	"github.com/vmware/vic/pkg/trace"
)

// Stripper strips the end-of-archive entries from a tar stream
type Stripper struct {
	// op allows threaded tracing
	op trace.Operation

	// be opinionated about the type of the source
	source *tar.Reader

	// close function to call if treated as io.Closer
	closer func() error
}

// NewStripper returns a WriterTo that will strip the trailing end-of-archive bytes
// from the supplied tar stream.
// It implements io.Reader only so that it can be passed to io.Copy
func NewStripper(op trace.Operation, reader *tar.Reader, close func() error) *Stripper {
	return &Stripper{
		op:     op,
		source: reader,
		closer: close,
	}
}

// Read is implemented solely so this can be provided to io.Copy as an io.Reader.
// This works on the assumption of io.Copy making use of the WriterTo implementation.
func (s *Stripper) Read(b []byte) (int, error) {
	panic("io.Reader usage not supported - intended use is as io.WriterTo")
}

// WriteTo is the primary function, allowing easy use of the underlying tar stream without
// requiring chunking and assocated tracking to another buffer size.
// Of note is that this returns the number of DATA bytes written, excluding the header bytes.
func (s *Stripper) WriteTo(w io.Writer) (sum int64, err error) {
	// TODO: should we nil s.source on error then handle a post-error call? What's the expected
	// semantic?

	tw := tar.NewWriter(w)
	for {
		var header *tar.Header
		header, err = s.source.Next()
		if err == io.EOF {
			// do NOT call tarwriter.Close() and drop the EOF for io.Copy behaviour
			err = nil
			s.op.Debugf("Stripper dropping end of archive")
			return
		}

		if err != nil {
			s.op.Errorf("Error reading archive header: %s", err)
			return
		}

		err = tw.WriteHeader(header)
		if err != nil {
			s.op.Errorf("Error writing tar header: %s", err)
			return
		}

		var n int64
		n, err = io.Copy(tw, s.source)
		sum += n
		if err != nil {
			s.op.Errorf("Error copying file data: %s", err)
			return
		}

		// #nosec: Errors unhandled.
		tw.Flush()
	}
}

// Close allows us to proxy a close on the stripper to the wrapped input
func (s *Stripper) Close() error {
	if s.closer != nil {
		s.op.Debugf("Closing stripper source: %p", s)
		return s.closer()
	}

	return nil
}

// eofReader copied from io package to support MultiWriterTo variant
type eofReader struct{}

func (eofReader) WriteTo(w io.Writer) (int64, error) {
	return 0, io.EOF
}

func (eofReader) Read(b []byte) (int, error) {
	return 0, io.EOF
}

func (eofReader) ReadFrom(r io.Reader) (int64, error) {
	return 0, io.EOF
}

// multiStripper based off io.MultiReader but delegating to io.WriterTo
// instead of performing buffer copy
type multiReader struct {
	readers []io.Reader
}

func (mr *multiReader) Read(p []byte) (n int, err error) {
	panic("io.Reader usage not supported - intended use is as io.WriterTo")
}

func (mr *multiReader) WriteTo(w io.Writer) (sum int64, err error) {
	for _, reader := range mr.readers {
		var n int64

		n, err = io.Copy(w, reader)
		sum += n

		// io.Copy never returns EOF so treat nil as EOF but keeping
		// EOF for clarity
		if err == io.EOF || err == nil {
			continue
		}

		// err was non-nil/EOF and we read data - legitimate error scenario
		if n > 0 {
			return
		}
	}

	err = nil
	return
}

// Close allows this to be a Closer as well - specific to expected usage but necessary.
func (mr *multiReader) Close() error {
	for _, r := range mr.readers {
		// if it's a closer, close it
		if closer, ok := r.(io.Closer); ok {
			// #nosec: Errors unhandled.
			closer.Close()
		}
	}

	return nil
}

// MultiReader is based off the io.MultiReader but will make use of WriteTo or
// ReadFrom delegation and ONLY supports usage via the WriteTo method on itself.
// It is specifically intended to be passed to io.Copy
func MultiReader(readers ...io.Reader) io.ReadCloser {
	r := make([]io.Reader, len(readers))
	copy(r, readers)
	return &multiReader{r}
}
