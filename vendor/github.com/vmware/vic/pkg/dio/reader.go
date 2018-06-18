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

// Package dio adds dynamic behaviour to the standard io package mutliX types
package dio

import (
	"io"
	"sync"

	log "github.com/Sirupsen/logrus"
)

// DynamicMultiReader adds dynamic add/remove to the base multireader behaviour
type DynamicMultiReader interface {
	io.Reader
	Add(...io.Reader)
	Remove(io.Reader)
	Close() error
	PropagateEOF(bool)
}

type multiReader struct {
	mutex sync.Mutex

	cond           *sync.Cond
	err            error
	readers        []io.Reader
	honorInlineEOF bool
}

// PropagateEOF toggles whether to return EOF when all readers return EOF.
// Setting this to true will result in an EOF if there are no readers available
// when Read is next called
func (t *multiReader) PropagateEOF(val bool) {
	t.mutex.Lock()
	t.honorInlineEOF = val
	t.cond.Broadcast()
	t.mutex.Unlock()
}

func (t *multiReader) Read(p []byte) (int, error) {
	var n int
	var err error
	var rTmp []io.Reader

	if verbose {
		defer func() {
			log.Debugf("[%p] read %q from %d readers (err: %#+v)", t, string(p[:n]), len(rTmp), err)
		}()
	}

	t.mutex.Lock()
	// stash a copy of the t.err
	err = t.err
	t.mutex.Unlock()

	// Close sets this
	if err == io.EOF || err == io.ErrClosedPipe {
		if verbose {
			log.Debugf("[%p] read from closed multi-reader, returning EOF", t)
		}
		return 0, io.EOF
	}

	// if there's no readers we are steady state - has to be after t.err check to
	// get correct Close behaviour.
	// Blocking behaviour!
	t.mutex.Lock()
	for len(t.readers) == 0 && t.err == nil {
		log.Debugf("[%p] Going into sleep with %d readers", t, len(t.readers))
		t.cond.Wait()
		log.Debugf("[%p] Woken from sleep %d readers", t, len(t.readers))
	}

	// stash a copy of the readers slie to iterate later
	rTmp = make([]io.Reader, len(t.readers))
	copy(rTmp, t.readers)
	// stash a copy of the t.err
	err = t.err
	t.mutex.Unlock()

	if err != nil {
		return 0, err
	}

	// eof counter
	eof := 0
	for _, r := range rTmp {
		slice := p[n:]
		if len(slice) == 0 {
			// we've run out of target space and don't know what
			// the remaining readers have, so not EOF
			return n, nil
		}

		x, err := r.Read(slice)
		n += x
		if err != nil {
			if err != io.EOF && err != io.ErrClosedPipe {
				t.mutex.Lock()
				// if there was an actual error, return that
				t.err = err
				t.mutex.Unlock()

				return n, err
			}
			// increment the EOF counter and remove the reader that retured EOF
			log.Debugf("[%p] removing reader due to EOF", t)
			// Remove grabs the lock
			t.Remove(r)

			eof++
		}
	}

	// This means readers closed/removed while we iterate
	// if no data is to be returned, there's no major error, and the number of
	// reported EOFs matches the number of readers on entry to the main loop
	if n == 0 && t.err == nil && eof == len(rTmp) {
		log.Debugf("[%p] All of the readers returned EOF (%d)", t, len(rTmp))
		t.mutex.Lock()
		// queue up an EOF for the next time around if no new readers are added
		if t.honorInlineEOF {
			t.err = io.EOF
		}
		t.mutex.Unlock()
	}
	return n, nil
}

func (t *multiReader) Add(reader ...io.Reader) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.readers = append(t.readers, reader...)
	// if we've got a new reader, we're not EOF any more until that reader EOFs
	t.err = nil
	t.cond.Broadcast()

	if verbose {
		log.Debugf("[%p] added reader - now %d readers", t, len(t.readers))

		for i, r := range t.readers {
			log.Debugf("[%p] Reader %d [%p]", t, i, r)
		}
	}
}

// TODO: add a WriteTo for more efficient copy

func (t *multiReader) Close() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	log.Debugf("[%p] Close on readers", t)
	for _, r := range t.readers {
		if c, ok := r.(io.Closer); ok {
			log.Debugf("[%p] Closing reader %+v", t, r)
			c.Close()
		}
	}

	t.err = io.EOF
	t.cond.Broadcast()
	return nil
}

// Remove doesn't return an error if element isn't found as the end result is
// identical
func (t *multiReader) Remove(reader io.Reader) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if verbose {
		log.Debugf("[%p] removing reader - currently %d readers", t, len(t.readers))
	}

	for i, r := range t.readers {
		if r == reader {
			t.readers = append(t.readers[:i], t.readers[i+1:]...)
			// using range directly means that we're looping up, so indexes are now invalid
			if verbose {
				log.Debugf("[%p] removed reader - now %d readers", t, len(t.readers))

				for i, r := range t.readers {
					log.Debugf("[%p] Reader %d [%p]", t, i, r)
				}
			}
			break
		}
	}
}

// MultiReader returns a Reader that's the logical concatenation of
// the provided input readers.  They're read sequentially.  Once all
// inputs have returned EOF, Read will return EOF.  If any of the readers
// return a non-nil, non-EOF error, Read will return that error.
func MultiReader(readers ...io.Reader) DynamicMultiReader {
	r := make([]io.Reader, len(readers))
	copy(r, readers)
	t := &multiReader{readers: r}
	t.cond = sync.NewCond(&t.mutex)

	if verbose {
		log.Debugf("[%p] created multireader", t)
	}
	return t
}
