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

package etcconf

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"
)

type EntryConsumer interface {
	ConsumeEntry(string) error
}

type EntryWalker interface {
	HasNext() bool
	Next() string
}

type Conf interface {
	Copy(Conf) error
	Load() error
	Save() error
	Path() string
}

func load(filePath string, con EntryConsumer) error {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Infof("not loading file %s as it does not exist", filePath)
			return nil
		}

		return err
	}
	// #nosec: Errors unhandled
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		t := strings.TrimSpace(s.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}

		if err = con.ConsumeEntry(t); err != nil {
			return err
		}
	}

	return nil
}

func save(filePath string, walker EntryWalker) error {
	f, err := ioutil.TempFile(path.Dir(filePath), path.Base(filePath))
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())

	w := bufio.NewWriter(f)
	for walker.HasNext() {
		l := fmt.Sprintf("%s\n", walker.Next())
		log.Debugf("writing %q to %s", l, filePath)
		var n int
		for n < len(l) {
			b, err := w.WriteString(l[n:])
			if err != nil {
				// #nosec: Errors unhandled
				_ = f.Close()
				return err
			}

			n += b
		}
	}

	if err = w.Flush(); err != nil {
		// #nosec: Errors unhandled
		_ = f.Close()
		log.Errorf("Unable to flush file content to %s: %s", f.Name(), err)
		return err
	}

	if err = f.Close(); err != nil {
		log.Errorf("Error closing file %s: %s", f.Name(), err)
		return err
	}

	if err = os.Rename(f.Name(), filePath); err != nil {
		log.Errorf("Unable to rename tempory file into final location %s->%s: %s", f.Name(), filePath, err)
		return err
	}

	return nil
}
