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
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/trace"
)

var (
	newDir, oldDir string
	a, b           []byte
	files          map[string][]string
	directories    map[string]struct{}
	err            error
)

func TestMain(m *testing.M) {

	var err error

	directories = make(map[string]struct{}, 4)
	files = make(map[string][]string, 6)
	files["original"] = []string{"file1", "file2", "file3", "file4",
		"original/file1", "original/file2", "original/remove",
		"exclude/excludeme", "exclude/includeme", "excludeme", "include/excludeme", "include/includeme"}
	files["added"] = []string{"added1", "added2", "add/file1", "add/file2"}
	files["changed"] = []string{"file1", "original/file2",
		"exclude/excludeme", "exclude/includeme",
		"include/excludeme", "include/includeme",
		"excludeme"}
	files["removed"] = []string{"file2", "original/file1", "original/remove"}
	files["excluded"] = []string{"exclude/", "excludeme", "include/excludeme"}
	files["included"] = []string{"exclude/includeme", "include/"}

	newDir, err = ioutil.TempDir("", "mnt")
	if err != nil {
		return
	}
	defer os.RemoveAll(newDir)

	oldDir, err = ioutil.TempDir("", "mnt")
	if err != nil {
		return
	}
	defer os.RemoveAll(oldDir)

	a = []byte("The mollusk lingers with its wandering eye\n")
	b = []byte("The waking of all creatures that live on the land\n")

	for _, dir := range []string{"original/", "add/", "exclude/", "include/"} {
		directories[dir] = struct{}{}
		if err = os.Mkdir(filepath.Join(oldDir, dir), 0777); err != nil {
			log.Errorf("Failed to add directory: %s", err.Error())
			return
		}
		if err = os.Mkdir(filepath.Join(newDir, dir), 0777); err != nil {
			log.Errorf("Failed to add directory: %s", err.Error())
			return
		}
	}

	for _, file := range files["original"] {
		if err = ioutil.WriteFile(filepath.Join(oldDir, file), a, 0777); err != nil {
			log.Errorf("Failed to add file: %s", err.Error())
			return
		}
		if err = ioutil.WriteFile(filepath.Join(newDir, file), a, 0777); err != nil {
			log.Errorf("Failed to add file: %s", err.Error())
			return
		}
	}
	for _, file := range append(files["added"], files["changed"]...) {
		if err = ioutil.WriteFile(filepath.Join(newDir, file), b, 0777); err != nil {
			log.Errorf("Failed to add file: %s", err.Error())
			return
		}
	}
	for _, file := range files["removed"] {
		if err = os.Remove(filepath.Join(newDir, file)); err != nil {
			log.Errorf("Failed to remove file: %s", err.Error())
			return
		}
	}

	os.Exit(m.Run())
}

func TestDiff(t *testing.T) {
	op := trace.NewOperation(context.Background(), "TestDiff")

	tarFile, err := Diff(op, newDir, oldDir, nil, true, true)
	if !assert.NoError(t, err) {
		return
	}

	tarredFiles := make(map[string][]byte)
	tr := tar.NewReader(tarFile)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			op.Errorf("Error reading tar archive: %s", err.Error())
		}

		buf := bytes.NewBuffer([]byte{})
		if _, err := io.Copy(buf, tr); !assert.NoError(t, err) {
			return
		}
		tarredFiles[hdr.Name] = buf.Bytes()
	}

	all := append(files["added"], append(files["changed"], files["included"]...)...)
	for _, file := range all {
		if strings.HasSuffix(file, "/") {
			continue
		}

		f, ok := tarredFiles[file]
		assert.True(t, ok, "Expected to find %s in tar archive", file)

		// don't try to check the contents if its a directory
		if _, ok := directories[file]; !ok {
			assert.Equal(t, b, f, "Expected file contents \"%s\", but found \"%s\"", b, f)
		}
	}
	for _, file := range files["removed"] {
		wh := filepath.Join(filepath.Dir(file), ".wh."+filepath.Base(file))
		_, ok := tarredFiles[wh]
		assert.True(t, ok, "Expected to find whiteout file in archive: %s", wh)
	}

}

func TestDiffNoAncestor(t *testing.T) {
	op := trace.NewOperation(context.Background(), "TestDiffNoParent")

	// test without ancestor
	tarFile, err := Diff(op, newDir, "", nil, true, true)
	if !assert.NoError(t, err) {
		return
	}

	tarredFiles := make(map[string][]byte)
	tr := tar.NewReader(tarFile)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			op.Errorf("Error reading tar header: %s", err.Error())
		}

		buf := bytes.NewBuffer([]byte{})
		if _, err := io.Copy(buf, tr); !assert.NoError(t, err) {
			return
		}
		tarredFiles[hdr.Name] = buf.Bytes()
		if hdr.Typeflag == tar.TypeDir {
			directories[hdr.Name] = struct{}{}
		}
	}

	all := append(files["added"], append(files["changed"], files["included"]...)...)
	for _, file := range all {
		f, ok := tarredFiles[file]
		assert.True(t, ok, "Expected to find %s in tar archive", file)

		// don't try to check the contents if its a directory
		if _, ok := directories[file]; !ok {
			assert.Equal(t, b, f, "Expected file contents \"%s\", but found \"%s\"", b, f)
		}
	}

	for _, file := range files["removed"] {
		wh := filepath.Join(filepath.Dir(file), ".wh."+filepath.Base(file))
		_, ok := tarredFiles[wh]
		assert.False(t, ok, "Expected not to find whiteout file in archive: %s", wh)
	}
}

func TestDiffNoData(t *testing.T) {
	op := trace.NewOperation(context.Background(), "TestDiffNoData")

	tarFile, err := Diff(op, newDir, oldDir, nil, false, true)
	if !assert.NoError(t, err) {
		return
	}

	tarredFiles := make(map[string]string)
	changeTypes := make(map[string]string)
	tr := tar.NewReader(tarFile)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if !assert.NoError(t, err) {
			op.Errorf("Error reading tar header: %s", err.Error())
		}

		buf := bytes.NewBuffer([]byte{})
		n, err := io.Copy(buf, tr)
		if !assert.NoError(t, err) {
			return
		}
		assert.EqualValues(t, 0, n, "Expected 0 bytes copied, got %d instead", n)
		tarredFiles[hdr.Name] = string(buf.Bytes())
		changeTypes[hdr.Name] = hdr.Xattrs[ChangeTypeKey]
	}

	for _, file := range files["added"] {
		f, ok := tarredFiles[file]
		ctype := changeTypes[file]
		assert.True(t, ok, "Expected to find %s in tar archive", file)

		// don't try to check the contents if its a directory
		if _, ok := directories[file]; !ok {
			assert.Equal(t, "", f, "Expected file contents \"%s\", but found \"%s\"", "", f)
			assert.Equal(t, "A", ctype, "Expected change type \"%s\", but found \"%s\"", "A", ctype)
		}
	}
	for _, file := range append(files["changed"], files["included"]...) {
		f, ok := tarredFiles[file]
		ctype := changeTypes[file]
		assert.True(t, ok, "expected to find %s in tar archive", file)
		// don't try to check the contents if its a directory
		if _, ok := directories[file]; !ok {
			assert.Equal(t, "", f, "expected file contents \"%s\", but found \"%s\"", "", f)
			assert.Equal(t, "C", ctype, "expected change type \"%s\", but found \"%s\"", "C", ctype)
		}
	}
	for _, file := range files["removed"] {
		wh := filepath.Join(filepath.Dir(file), ".wh."+filepath.Base(file))
		_, ok := tarredFiles[wh]
		ctype, cok := changeTypes[wh]
		assert.True(t, ok, "Expected to find whiteout file in archive: %s", wh)
		assert.True(t, cok, "Expected to find change type for %s", wh)
		assert.Equal(t, "D", ctype, "Expected change type \"%s\" but found \"%s\"", "D", ctype)
	}
}

func TestDiffFilterSpec(t *testing.T) {

	op := trace.NewOperation(context.Background(), "TestDiffFilterSpec")

	filter := make(map[string]FilterType)
	for _, path := range files["excluded"] {
		p := strings.TrimSuffix(path, "/")
		filter[p] = Exclude
	}
	for _, path := range files["included"] {
		filter[path] = Include
	}

	spec, err := CreateFilterSpec(op, filter)
	if !assert.NoError(t, err) {
		return
	}

	tarFile, err := Diff(op, newDir, oldDir, spec, true, true)
	if !assert.NoError(t, err) {
		return
	}

	tarredFiles := make(map[string][]byte)
	tr := tar.NewReader(tarFile)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			op.Errorf("Error reading tar archive: %s", err.Error())
		}

		buf := bytes.NewBuffer([]byte{})
		if _, err := io.Copy(buf, tr); !assert.NoError(t, err) {
			return
		}
		tarredFiles[hdr.Name] = buf.Bytes()
	}

	all := append(files["added"], files["included"]...)
	for _, file := range all {
		if file == string(filepath.Separator) {
			continue
		}
		f, ok := tarredFiles[file]
		assert.True(t, ok, "Expected to find %s in tar archive", file)

		// don't try to check the contents if its a directory
		if _, ok := directories[file]; !ok {
			assert.Equal(t, b, f, "Expected file contents \"%s\" for %s, but found \"%s\"", b, file, f)
		}
	}
	for _, file := range files["removed"] {
		wh := filepath.Join(filepath.Dir(file), ".wh."+filepath.Base(file))
		_, ok := tarredFiles[wh]
		assert.True(t, ok, "Expected to find whiteout file in archive: %s", wh)
	}
	for _, file := range files["excluded"] {
		_, ok := tarredFiles[file]
		assert.False(t, ok, "Expected excluded file to be missing from archive: %s", file)
	}
}

func TestDiffFilterSpecNoAncestor(t *testing.T) {

	op := trace.NewOperation(context.Background(), "TestDiffFilterSpecNoParent")

	filter := make(map[string]FilterType)
	for _, path := range files["excluded"] {
		p := strings.TrimSuffix(path, "/")
		filter[p] = Exclude
	}
	for _, path := range files["included"] {
		filter[path] = Include
	}

	spec, err := CreateFilterSpec(op, filter)
	if !assert.NoError(t, err) {
		return
	}

	// test without ancestor
	tarFile, err := Diff(op, newDir, "", spec, true, true)
	if !assert.NoError(t, err) {
		return
	}

	tarredFiles := make(map[string][]byte)
	tr := tar.NewReader(tarFile)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			op.Errorf("Error reading tar archive: %s", err.Error())
		}

		buf := bytes.NewBuffer([]byte{})
		if _, err := io.Copy(buf, tr); !assert.NoError(t, err) {
			return
		}
		tarredFiles[hdr.Name] = buf.Bytes()
	}

	all := append(files["added"], files["included"]...)
	for _, file := range all {
		f, ok := tarredFiles[file]
		if file == string(filepath.Separator) {
			continue
		}
		assert.True(t, ok, "Expected to find %s in tar archive", file)

		// don't try to check the contents if its a directory
		if _, ok := directories[file]; !ok {
			assert.Equal(t, b, f, "Expected file contents \"%s\", but found \"%s\" for target file (%s)", b, f, file)
		}
	}
	for _, file := range files["removed"] {
		wh := filepath.Join(filepath.Dir(file), ".wh."+filepath.Base(file))
		_, ok := tarredFiles[wh]
		assert.False(t, ok, "Expected not to find whiteout file in archive: %s", wh)
	}
	for _, file := range files["excluded"] {
		_, ok := tarredFiles[file]
		assert.False(t, ok, "Expected excluded file to be missing from archive: %s", file)
	}
}

func TestDiffFilterSpecRebase(t *testing.T) {

	op := trace.NewOperation(context.Background(), "TestDiffFilterSpecRebase")

	filter := make(map[string]FilterType)
	for _, path := range files["excluded"] {
		filter[path] = Exclude
	}
	for _, path := range files["included"] {
		filter[path] = Include
	}
	rebasePath := "path/to/prefix"
	filter[rebasePath] = Rebase

	spec, err := CreateFilterSpec(op, filter)
	if !assert.NoError(t, err) {
		return
	}

	// test without ancestor
	tarFile, err := Diff(op, newDir, oldDir, spec, true, true)
	if !assert.NoError(t, err) {
		return
	}

	tarredFiles := make(map[string][]byte)
	tr := tar.NewReader(tarFile)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			op.Errorf("Error reading tar archive: %s", err.Error())
		}

		buf := bytes.NewBuffer([]byte{})
		if _, err := io.Copy(buf, tr); !assert.NoError(t, err) {
			return
		}
		tarredFiles[strings.TrimSuffix(hdr.Name, "/")] = buf.Bytes()
	}

	all := append(files["added"], files["included"]...)
	for _, file := range all {
		if file == string(filepath.Separator) {
			continue
		}

		rebasedPath := filepath.Join(rebasePath, file)
		var isDir bool
		_, isDir = directories[file]

		f, ok := tarredFiles[rebasedPath]
		assert.True(t, ok, "Expected to find %s in tar archive", rebasedPath)
		if !isDir {
			assert.Equal(t, b, f, "Expected file contents \"%s\", but found \"%s\" for target file (%s)", b, f, rebasedPath)
		}
	}
	for _, file := range files["removed"] {
		wh := filepath.Join(filepath.Dir(file), ".wh."+filepath.Base(file))
		wh = filepath.Join(rebasePath, wh)
		_, ok := tarredFiles[wh]
		assert.True(t, ok, "Expected not to find whiteout file in archive: %s", wh)
	}
	for _, file := range files["excluded"] {
		_, ok := tarredFiles[file]
		assert.False(t, ok, "Expected excluded file to be missing from archive: %s", file)
	}
}

func TestDiffFilterSpecRebaseNoData(t *testing.T) {

	op := trace.NewOperation(context.Background(), "TestDiffFilterSpecRebase")

	filter := make(map[string]FilterType)
	for _, path := range files["excluded"] {
		filter[path] = Exclude
	}
	for _, path := range files["included"] {
		filter[path] = Include
	}
	rebasePath := "path/to/prefix"
	filter[rebasePath] = Rebase

	spec, err := CreateFilterSpec(op, filter)
	if !assert.NoError(t, err) {
		return
	}

	tarFile, err := Diff(op, newDir, oldDir, spec, false, true)
	if !assert.NoError(t, err) {
		return
	}

	tarredFiles := make(map[string]struct{})
	changeTypes := make(map[string]string)
	tr := tar.NewReader(tarFile)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if !assert.NoError(t, err) {
			op.Errorf("Error reading tar header: %s", err.Error())
		}

		buf := bytes.NewBuffer([]byte{})
		n, err := io.Copy(buf, tr)
		if !assert.NoError(t, err) {
			return
		}
		assert.EqualValues(t, 0, n, "Expected 0 bytes copied, got %d instead", n)
		tarredFiles[strings.TrimSuffix(hdr.Name, "/")] = struct{}{}
		changeTypes[hdr.Name] = hdr.Xattrs[ChangeTypeKey]
	}

	for _, file := range files["added"] {

		rebasedPath := filepath.Join(rebasePath, file)
		var isDir bool
		_, isDir = directories[file]

		_, ok := tarredFiles[rebasedPath]
		ctype := changeTypes[rebasedPath]
		assert.True(t, ok, "Expected to find %s in tar archive", file)

		// don't try to check the contents if its a directory
		if !isDir {
			assert.Equal(t, "A", ctype, "Expected change type \"%s\", but found \"%s\"", "A", ctype)
		}
	}
	for _, file := range append(files["changed"], files["included"]...) {

		// don't check for excludes or whiteouts yet
		base := filepath.Base(file)
		if strings.HasSuffix(file, "excludeme") || strings.HasPrefix(base, ".wh.") {
			continue
		}

		rebasedPath := filepath.Join(rebasePath, file)
		var isDir bool
		_, isDir = directories[file]
		_, ok := tarredFiles[rebasedPath]
		ctype := changeTypes[rebasedPath]
		assert.True(t, ok, "expected to find %s in tar archive", file)
		// don't try to check the contents if its a directory
		if !isDir {
			assert.Equal(t, "C", ctype, "expected change type \"%s\", but found \"%s\"", "C", ctype)
		}
	}
	for _, file := range files["removed"] {
		wh := filepath.Join(filepath.Dir(file), ".wh."+filepath.Base(file))
		wh = filepath.Join(rebasePath, wh)
		_, ok := tarredFiles[wh]
		ctype, cok := changeTypes[wh]
		assert.True(t, ok, "Expected to find whiteout file in archive: %s", wh)
		assert.True(t, cok, "Expected to find change type for %s", wh)
		assert.Equal(t, "D", ctype, "Expected change type \"%s\" but found \"%s\"", "D", ctype)
	}

	for _, file := range files["excluded"] {
		_, ok := tarredFiles[file]
		assert.False(t, ok, "Expected excluded file to be missing from archive: %s", file)
	}
}

func TestDiffCreateFilterSpec(t *testing.T) {
	op := trace.NewOperation(context.Background(), "TestDiffCreateFilterSpec")

	filter := make(map[string]FilterType)
	for _, path := range files["excluded"] {
		filter[path] = Exclude
	}
	for _, path := range files["included"] {
		filter[path] = Include
	}
	rebasePath := "/path/to/prefix"
	filter[rebasePath] = Rebase

	stripPath := "/path/to/strip"
	filter[stripPath] = Strip

	spec, err := CreateFilterSpec(op, filter)
	if !assert.NoError(t, err) {
		return
	}

	for _, path := range files["included"] {
		_, ok := spec.Inclusions[path]
		assert.True(t, ok, "Expected to find %s in inclusions set", path)
		_, ok = spec.Exclusions[path]
		assert.False(t, ok, "Expected not to find %s in exclusions set", path)
	}

	for _, path := range files["excluded"] {
		_, ok := spec.Exclusions[path]
		assert.True(t, ok, "Expected to find %s in exclusions set", path)
		_, ok = spec.Inclusions[path]
		assert.False(t, ok, "Expected not to find %s in inclusions set", path)
	}

	assert.Equal(t, rebasePath, spec.RebasePath)
	assert.Equal(t, stripPath, spec.StripPath)
	e := "/path/to/extra/rebase"
	filter[e] = Rebase

	spec, err = CreateFilterSpec(op, filter)
	assert.Nil(t, spec, "Expected nil spec")
	assert.Error(t, err)
	assert.EqualError(t, err, "error creating filter spec: only one rebase path allowed")

	delete(filter, e)

	s := "/path/to/extra/strippath"
	filter[s] = Strip
	spec, err = CreateFilterSpec(op, filter)
	assert.Nil(t, spec, "Expected nil spec")
	assert.Error(t, err)
	assert.EqualError(t, err, "error creating filter spec: only one strip path allowed")

	delete(filter, s)

	filter[s] = 20
	spec, err = CreateFilterSpec(op, filter)
	assert.Nil(t, spec, "Expected nil spec")
	assert.Error(t, err)
	assert.EqualError(t, err, "invalid filter specification: 20")
}
