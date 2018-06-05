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

package archive

import (
	"archive/tar"
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/trace"
)

var (
	// L1 directories
	path1dir1 = "/path1dir1"
	path2dir1 = "/path2dir1"

	// l1 files
	l1file1 = "/file1"
	l1file2 = "/file2"

	// l2 directories
	path1dir2 = filepath.Join(path1dir1, "path1dir2")
	path1dir3 = filepath.Join(path1dir1, "path1dir3")
	path2dir2 = filepath.Join(path2dir1, "path2dir2")
	path2dir3 = filepath.Join(path2dir1, "path2dir3")

	// l2 files
	l2file1 = filepath.Join(path1dir1, "file1")
	l2file2 = filepath.Join(path1dir1, "file2")
	l2file3 = filepath.Join(path2dir1, "file1")
	l2file4 = filepath.Join(path2dir1, "file2")

	// l3 directories
	path1dir4 = filepath.Join(path1dir3, "path1dir4")

	// l3 files
	l3file1 = filepath.Join(path1dir3, "file1")
	l3file2 = filepath.Join(path2dir2, "file1")
)

func TestInvalidStream(t *testing.T) {
	op := trace.NewOperation(context.TODO(), "")

	// assemble an invalid stream
	var invalidBuffer *bytes.Buffer
	invalidBuffer = bytes.NewBufferString("I am not a tar stream, I am just a simple string.")
	tarStream := bytes.NewReader(invalidBuffer.Bytes())

	tempPath, err := ioutil.TempDir("", "write-unit-test")
	// just in case this test fails and attempts to write to the filesystem
	defer os.RemoveAll(tempPath)
	if !assert.NoError(t, err) {
		return
	}

	op.Infof("%s", tempPath)

	writetarget := "/"
	specMap := make(map[string]FilterType)
	specMap[writetarget] = Rebase

	filterSpec, err := CreateFilterSpec(op, specMap)
	if !assert.NoError(t, err) {
		return
	}
	err = UnpackNoChroot(op, tarStream, filterSpec, tempPath)

	if !assert.Error(t, err) {
		return
	}

	validEmptyBuffer := bytes.NewBufferString("")
	tarStream = bytes.NewReader(validEmptyBuffer.Bytes())

	err = UnpackNoChroot(op, tarStream, filterSpec, tempPath)

	if !assert.NoError(t, err) {
		return
	}
}

func TestSimpleWrite(t *testing.T) {
	op := trace.NewOperation(context.TODO(), "")

	filesToWrite := prepareTarFileSlice()
	tarBytes, err := TarFiles(filesToWrite)

	if !assert.NoError(t, err) {
		return
	}

	tarStream := ioutil.NopCloser(tarBytes)

	tempPath, err := ioutil.TempDir("", "write-unit-test")
	defer os.RemoveAll(tempPath)
	if !assert.NoError(t, err) {
		return
	}

	op.Infof("%s", tempPath)

	writetarget := "/"
	specMap := make(map[string]FilterType)
	specMap[writetarget] = Rebase

	filterSpec, err := CreateFilterSpec(op, specMap)
	if !assert.NoError(t, err) {
		return
	}
	err = UnpackNoChroot(op, tarStream, filterSpec, tempPath)

	if !assert.NoError(t, err) {
		return
	}

	for _, file := range filesToWrite {
		pathToFile := filepath.Join(tempPath, writetarget, file.Name)
		_, err = os.Stat(pathToFile)

		if err != nil {
			op.Infof("error for (%s) is (%s) ", file.Name, err.Error())
		}

		// In this test every file should have been written to the filesystem
		if !assert.NoError(t, err) {
			return
		}
	}

}

func TestSimpleWriteSymLink(t *testing.T) {
	op := trace.NewOperation(context.TODO(), "")

	writetarget := "/"
	tempPath, err := ioutil.TempDir("", "write-unit-test")
	defer os.RemoveAll(tempPath)
	if !assert.NoError(t, err) {
		return
	}

	filesToWrite := prepareTarFileSlice()

	symLinks := []TarFile{}
	// assemble standard file symlink
	symlinkName := filepath.Join(path1dir1, "link1")
	symlinkBody := filepath.Join("../", l1file1)
	symLink := TarFile{
		Name: symlinkName,
		Type: tar.TypeSymlink,
		Body: symlinkBody,
	}
	symLinks = append(symLinks, symLink)

	// assemble broken symlink
	brokenlinkName := filepath.Join(path1dir1, "BrokenSymLink")
	brokenlinkBody := filepath.Join("../DOES_NOT_EXIST")
	brokenLink := TarFile{
		Name: brokenlinkName,
		Type: tar.TypeSymlink,
		Body: brokenlinkBody,
	}
	symLinks = append(symLinks, brokenLink)

	dirlinkName := filepath.Join(path1dir1, "DirSymLink")
	dirlinkBody := filepath.Join("../", path1dir1)
	dirLink := TarFile{
		Name: dirlinkName,
		Type: tar.TypeSymlink,
		Body: dirlinkBody,
	}
	symLinks = append(symLinks, dirLink)
	op.Infof("Assembled list of test symlinks : (%s)", symLinks)
	filesToWrite = append(filesToWrite, symLinks...)

	tarBytes, err := TarFiles(filesToWrite)
	if !assert.NoError(t, err) {
		return
	}
	tarStream := ioutil.NopCloser(tarBytes)

	op.Infof("%s", tempPath)

	specMap := make(map[string]FilterType)
	specMap[writetarget] = Rebase

	filterSpec, err := CreateFilterSpec(op, specMap)
	if !assert.NoError(t, err) {
		return
	}

	err = UnpackNoChroot(op, tarStream, filterSpec, tempPath)

	if !assert.NoError(t, err) {
		return
	}

	// expect file not found for broken links, assemble list.
	NotExist := map[string]struct{}{
		brokenlinkName: {},
	}

	for _, file := range filesToWrite {
		pathToFile := filepath.Join(tempPath, writetarget, file.Name)
		_, err = os.Stat(pathToFile)

		if err != nil {
			if _, ok := NotExist[file.Name]; ok && os.IsNotExist(err) {
				// we expect and EOF only for the broken links in this test
				continue
			}
			op.Infof("error for (%s) is (%s) ", file.Name, err.Error())
		}

		// In this test every file should have been written to the filesystem
		if !assert.NoError(t, err) {
			return
		}
	}

}

func TestSimpleWriteSymLinkNonRootTarget(t *testing.T) {
	op := trace.NewOperation(context.TODO(), "")

	writetarget := "/nonRootPath"
	tempPath, err := ioutil.TempDir("", "write-unit-test")
	defer os.RemoveAll(tempPath)
	if !assert.NoError(t, err) {
		return
	}

	filesToWrite := prepareTarFileSlice()

	symLinks := []TarFile{}
	// assemble standard file symlink
	symlinkName := filepath.Join(path1dir1, "link1")
	symlinkBody := filepath.Join("../", l1file1)
	symLink := TarFile{
		Name: symlinkName,
		Type: tar.TypeSymlink,
		Body: symlinkBody,
	}
	symLinks = append(symLinks, symLink)

	// assemble broken symlink
	brokenlinkName := filepath.Join(path1dir1, "BrokenSymLink")
	brokenlinkBody := filepath.Join("../DOES_NOT_EXIST")
	brokenLink := TarFile{
		Name: brokenlinkName,
		Type: tar.TypeSymlink,
		Body: brokenlinkBody,
	}
	symLinks = append(symLinks, brokenLink)

	dirlinkName := filepath.Join(path1dir1, "DirSymLink")
	dirlinkBody := filepath.Join("../", path1dir1)
	dirLink := TarFile{
		Name: dirlinkName,
		Type: tar.TypeSymlink,
		Body: dirlinkBody,
	}
	symLinks = append(symLinks, dirLink)
	op.Infof("Assembled list of test symlinks : (%s)", symLinks)
	filesToWrite = append(filesToWrite, symLinks...)

	tarBytes, err := TarFiles(filesToWrite)
	if !assert.NoError(t, err) {
		return
	}
	tarStream := ioutil.NopCloser(tarBytes)

	op.Infof("%s", tempPath)

	specMap := make(map[string]FilterType)
	specMap[writetarget] = Rebase

	filterSpec, err := CreateFilterSpec(op, specMap)
	if !assert.NoError(t, err) {
		return
	}

	err = UnpackNoChroot(op, tarStream, filterSpec, tempPath)

	if !assert.NoError(t, err) {
		return
	}

	// expect file not found for broken links, assemble list.
	NotExist := map[string]struct{}{
		brokenlinkName: {},
	}

	for _, file := range filesToWrite {
		pathToFile := filepath.Join(tempPath, writetarget, file.Name)

		// it is important to note for this test that the behavior of os.Stat
		// is to report the type of the target of a symlink. unless the link is broken.
		_, err = os.Stat(pathToFile)

		if err != nil {
			if _, ok := NotExist[file.Name]; ok && os.IsNotExist(err) {
				// we expect and EOF only for the broken links in this test
				continue
			}
			op.Infof("error for (%s) is (%s) ", file.Name, err.Error())
		}

		// In this test every file should have been written to the filesystem
		if !assert.NoError(t, err) {
			return
		}
	}

}

func TestSimpleWriteNonRootTarget(t *testing.T) {
	op := trace.NewOperation(context.TODO(), "")

	filesToWrite := prepareTarFileSlice()
	tarBytes, err := TarFiles(filesToWrite)

	if !assert.NoError(t, err) {
		return
	}

	tarStream := ioutil.NopCloser(tarBytes)

	tempPath, err := ioutil.TempDir("", "write-unit-test")
	defer os.RemoveAll(tempPath)
	if !assert.NoError(t, err) {
		return
	}

	op.Infof("%s", tempPath)

	writetarget := "/data/target"
	specMap := make(map[string]FilterType)
	specMap[writetarget] = Rebase

	filterSpec, err := CreateFilterSpec(op, specMap)
	if !assert.NoError(t, err) {
		return
	}
	err = UnpackNoChroot(op, tarStream, filterSpec, tempPath)

	if !assert.NoError(t, err) {
		return
	}

	for _, file := range filesToWrite {
		pathToFile := filepath.Join(tempPath, writetarget, file.Name)
		_, err = os.Stat(pathToFile)

		if err != nil {
			op.Infof("error for (%s) is (%s) ", file.Name, err.Error())
		}

		// In this test every file should have been written to the filesystem
		if !assert.NoError(t, err) {
			return
		}
	}

}

func TestSimpleExclusion(t *testing.T) {
	op := trace.NewOperation(context.TODO(), "")

	filesToWrite := prepareTarFileSlice()
	tarBytes, err := TarFiles(filesToWrite)

	if !assert.NoError(t, err) {
		return
	}

	tarStream := ioutil.NopCloser(tarBytes)

	tempPath, err := ioutil.TempDir("", "write-unit-test")
	defer os.RemoveAll(tempPath)
	if !assert.NoError(t, err) {
		return
	}

	op.Infof("%s", tempPath)

	exclusions := []string{
		path2dir1,
	}

	writetarget := "/"
	specMap := make(map[string]FilterType)
	specMap[writetarget] = Rebase

	for _, v := range exclusions {
		specMap[v] = Exclude
	}

	filterSpec, err := CreateFilterSpec(op, specMap)
	if !assert.NoError(t, err) {
		return
	}
	err = UnpackNoChroot(op, tarStream, filterSpec, tempPath)

	if !assert.NoError(t, err) {
		return
	}

	// record the expected excluded items for checking.
	expectedExcluded := map[string]struct{}{
		path2dir1: {},
		path2dir2: {},
		path2dir3: {},
		l2file3:   {},
		l2file4:   {},
		l3file2:   {},
	}

	for _, file := range filesToWrite {
		pathToFile := filepath.Join(tempPath, writetarget, file.Name)
		_, err = os.Stat(pathToFile)

		if err != nil {

			if os.IsNotExist(err) {
				if _, ok := expectedExcluded[file.Name]; ok {
					err = nil
				} else {
					// we did not want to exclude this but it was
				}
			} else {

				// some other path error occurred, record it for test purposes
				op.Infof("error for (%s) is (%s) ", file.Name, err.Error())
			}

		}

		if !assert.NoError(t, err) {
			return
		}
	}

}

func TestInclusionAfterExclusion(t *testing.T) {
	t.Skip("skipping until filterspec stabilizes and this is updated to reflect final form")

	op := trace.NewOperation(context.TODO(), "")

	filesToWrite := prepareTarFileSlice()
	tarBytes, err := TarFiles(filesToWrite)

	if !assert.NoError(t, err) {
		return
	}

	tarStream := ioutil.NopCloser(tarBytes)

	tempPath, err := ioutil.TempDir("", "write-unit-test")
	defer os.RemoveAll(tempPath)
	if !assert.NoError(t, err) {
		return
	}

	op.Infof("%s", tempPath)

	exclusions := []string{
		path2dir1,
	}

	inclusions := []string{
		l3file2,
	}

	writetarget := "/"
	specMap := make(map[string]FilterType)
	specMap[writetarget] = Rebase

	for _, v := range exclusions {
		specMap[v] = Exclude
	}

	for _, v := range inclusions {
		specMap[v] = Include
	}

	filterSpec, err := CreateFilterSpec(op, specMap)
	if !assert.NoError(t, err) {
		return
	}
	err = UnpackNoChroot(op, tarStream, filterSpec, tempPath)

	if !assert.NoError(t, err) {
		return
	}

	// record the expected excluded items for checking.
	expectedExcluded := map[string]struct{}{
		path2dir1: {},
		path2dir3: {},
		l2file3:   {},
		l2file4:   {},
	}

	for _, file := range filesToWrite {
		pathToFile := filepath.Join(tempPath, writetarget, file.Name)
		_, err = os.Stat(pathToFile)

		if err != nil {

			if os.IsNotExist(err) {
				if _, ok := expectedExcluded[file.Name]; ok {
					err = nil
				} else {
					// we did not want to exclude this but it was
				}
			} else {

				// some other path error occurred, record it for test purposes
				op.Infof("error for (%s) is (%s) ", file.Name, err.Error())
			}

		}

		if !assert.NoError(t, err) {
			return
		}
	}

}

func TestMultiExclusion(t *testing.T) {
	op := trace.NewOperation(context.TODO(), "")

	filesToWrite := prepareTarFileSlice()
	tarBytes, err := TarFiles(filesToWrite)

	if !assert.NoError(t, err) {
		return
	}

	tarStream := ioutil.NopCloser(tarBytes)

	tempPath, err := ioutil.TempDir("", "write-unit-test")
	defer os.RemoveAll(tempPath)
	if !assert.NoError(t, err) {
		return
	}

	op.Infof("%s", tempPath)

	exclusions := []string{
		path2dir2,
		path1dir3,
	}

	writetarget := "/"
	specMap := make(map[string]FilterType)
	specMap[writetarget] = Rebase

	for _, v := range exclusions {
		specMap[v] = Exclude
	}

	filterSpec, err := CreateFilterSpec(op, specMap)
	if !assert.NoError(t, err) {
		return
	}
	err = UnpackNoChroot(op, tarStream, filterSpec, tempPath)

	if !assert.NoError(t, err) {
		return
	}

	// record the expected excluded items for checking.
	expectedExcluded := map[string]struct{}{

		// first exclusion
		path2dir2: {},
		l3file2:   {},

		// second exclusion
		path1dir3: {},
		path1dir4: {},
		l3file1:   {},
	}

	for _, file := range filesToWrite {
		pathToFile := filepath.Join(tempPath, writetarget, file.Name)
		_, err = os.Stat(pathToFile)

		if err != nil {

			if os.IsNotExist(err) {
				if _, ok := expectedExcluded[file.Name]; ok {
					err = nil
				} else {
					// we did not want to exclude this but it was
				}
			} else {

				// some other path error occurred, record it for test purposes
				op.Infof("error for (%s) is (%s) ", file.Name, err.Error())
			}

		}

		if !assert.NoError(t, err) {
			return
		}
	}

}

func TestMultiExclusionMultiInclusion(t *testing.T) {
	t.Skip("skipping until filterspec stabilizes and this is updated to reflect final form")

	op := trace.NewOperation(context.TODO(), "")

	filesToWrite := prepareTarFileSlice()
	tarBytes, err := TarFiles(filesToWrite)

	if !assert.NoError(t, err) {
		return
	}

	tarStream := ioutil.NopCloser(tarBytes)

	tempPath, err := ioutil.TempDir("", "write-unit-test")
	defer os.RemoveAll(tempPath)
	if !assert.NoError(t, err) {
		return
	}

	op.Infof("%s", tempPath)

	exclusions := []string{
		path2dir2,
		path1dir3,
	}

	inclusions := []string{
		l3file1,
		l3file2,
	}

	writetarget := "/"
	specMap := make(map[string]FilterType)
	specMap[writetarget] = Rebase

	for _, v := range exclusions {
		specMap[v] = Exclude
	}

	for _, v := range inclusions {
		specMap[v] = Include
	}

	filterSpec, err := CreateFilterSpec(op, specMap)
	if !assert.NoError(t, err) {
		return
	}
	err = UnpackNoChroot(op, tarStream, filterSpec, tempPath)

	if !assert.NoError(t, err) {
		return
	}

	// record the expected excluded items for checking.
	expectedExcluded := map[string]struct{}{
		path1dir4: {},
	}

	for _, file := range filesToWrite {
		pathToFile := filepath.Join(tempPath, writetarget, file.Name)
		_, err = os.Stat(pathToFile)

		if err != nil {

			if os.IsNotExist(err) {
				if _, ok := expectedExcluded[file.Name]; ok {
					err = nil
				} else {
					// we did not want to exclude this but it was
				}
			} else {

				// some other path error occurred, record it for test purposes
				op.Infof("error for (%s) is (%s) ", file.Name, err.Error())
			}

		}

		if !assert.NoError(t, err) {
			return
		}
	}

}

func TestMultiExclusionMultiInclusionDirectories(t *testing.T) {
	t.Skip("skipping until filterspec stabilizes and this is updated to reflect final form")

	op := trace.NewOperation(context.TODO(), "")

	filesToWrite := prepareTarFileSlice()
	tarBytes, err := TarFiles(filesToWrite)

	if !assert.NoError(t, err) {
		return
	}

	tarStream := ioutil.NopCloser(tarBytes)

	tempPath, err := ioutil.TempDir("", "write-unit-test")
	defer os.RemoveAll(tempPath)
	if !assert.NoError(t, err) {
		return
	}

	op.Infof("%s", tempPath)

	exclusions := []string{
		path2dir1,
		path1dir1,
	}

	inclusions := []string{
		path1dir3,
		path2dir2,
	}

	writetarget := "/"
	specMap := make(map[string]FilterType)
	specMap[writetarget] = Rebase

	for _, v := range exclusions {
		specMap[v] = Exclude
	}

	for _, v := range inclusions {
		specMap[v] = Include
	}

	filterSpec, err := CreateFilterSpec(op, specMap)
	if !assert.NoError(t, err) {
		return
	}
	err = UnpackNoChroot(op, tarStream, filterSpec, tempPath)

	if !assert.NoError(t, err) {
		return
	}

	// record the expected excluded items for checking.
	expectedExcluded := map[string]struct{}{
		// expected directory exclusions
		path1dir1: {},
		path1dir2: {},
		path2dir1: {},
		path2dir3: {},

		// expected file exclusions
		l2file1: {},
		l2file2: {},
		l2file3: {},
		l2file4: {},
	}

	for _, file := range filesToWrite {
		pathToFile := filepath.Join(tempPath, writetarget, file.Name)
		_, err = os.Stat(pathToFile)

		if err != nil {

			if os.IsNotExist(err) {
				if _, ok := expectedExcluded[file.Name]; ok {
					err = nil
				} else {
					// we did not want to exclude this but it was
				}
			} else {

				// some other path error occurred, record it for test purposes
				op.Infof("error for (%s) is (%s) ", file.Name, err.Error())
			}

		}

		if !assert.NoError(t, err) {
			return
		}
	}

}

func TestMultiExclusionMultiInclusionDirectoriesNonRootTarget(t *testing.T) {
	t.Skip("skipping until filterspec stabilizes and this is updated to reflect final form")

	op := trace.NewOperation(context.TODO(), "")

	filesToWrite := prepareTarFileSlice()
	tarBytes, err := TarFiles(filesToWrite)

	if !assert.NoError(t, err) {
		return
	}

	tarStream := ioutil.NopCloser(tarBytes)

	tempPath, err := ioutil.TempDir("", "write-unit-test")
	defer os.RemoveAll(tempPath)
	if !assert.NoError(t, err) {
		return
	}

	op.Infof("%s", tempPath)

	exclusions := []string{
		path2dir1,
		path1dir1,
	}

	inclusions := []string{
		path1dir3,
		path2dir2,
	}

	writetarget := "/data/target"
	specMap := make(map[string]FilterType)
	specMap[writetarget] = Rebase

	for _, v := range exclusions {
		specMap[v] = Exclude
	}

	for _, v := range inclusions {
		specMap[v] = Include
	}

	filterSpec, err := CreateFilterSpec(op, specMap)
	if !assert.NoError(t, err) {
		return
	}
	err = UnpackNoChroot(op, tarStream, filterSpec, tempPath)

	if !assert.NoError(t, err) {
		return
	}

	// record the expected excluded items for checking.
	expectedExcluded := map[string]struct{}{
		// expected directory exclusions
		path1dir1: {},
		path1dir2: {},
		path2dir1: {},
		path2dir3: {},

		// expected file exclusions
		l2file1: {},
		l2file2: {},
		l2file3: {},
		l2file4: {},
	}

	for _, file := range filesToWrite {
		pathToFile := filepath.Join(tempPath, writetarget, file.Name)
		_, err = os.Stat(pathToFile)

		if err != nil {

			if os.IsNotExist(err) {
				if _, ok := expectedExcluded[file.Name]; ok {
					err = nil
				} else {
					// we did not want to exclude this but it was
				}
			} else {

				// some other path error occurred, record it for test purposes
				op.Infof("error for (%s) is (%s) ", file.Name, err.Error())
			}

		}

		if !assert.NoError(t, err) {
			return
		}
	}

}

type bufWriteCloser struct {
	*bytes.Buffer
}

func newBufWriteCloser() *bufWriteCloser {
	b := new(bytes.Buffer)
	return &bufWriteCloser{Buffer: b}
}

func (b *bufWriteCloser) Write(p []byte) (n int, err error) {
	return b.Buffer.Write(p)
}

func (b *bufWriteCloser) Close() error {
	return nil
}

func TestStreamCopy(t *testing.T) {
	filesToWrite := prepareTarFileSlice()
	tarBytes, err := TarFiles(filesToWrite)
	assert.NoError(t, err)

	op := trace.NewOperation(context.TODO(), "")
	op.Infof("%d", len(tarBytes.Bytes()))

	foo := newBufWriteCloser()
	tr := tar.NewReader(tarBytes)
	err = streamCopy(op, foo, tr)

	assert.NoError(t, err)
	tarBytes, err = TarFiles(filesToWrite)
	tb := tarBytes.Bytes()

	assert.NoError(t, err)
	assert.NotEqual(t, len(tb), 0)
	assert.NotEqual(t, len(foo.Bytes()), 0)
	for i, b := range foo.Bytes() {
		assert.Equal(t, b, tb[i])
	}

	reader := new(bytes.Buffer)
	reader.Write([]byte("foobar"))
	foo = newBufWriteCloser()
	err = streamCopy(op, foo, reader)
	assert.NoError(t, err)

	tb = []byte("foobar")
	assert.NoError(t, err)
	assert.NotEqual(t, len(tb), 0)
	assert.NotEqual(t, len(foo.Bytes()), 0)
	for i, b := range foo.Bytes() {
		assert.Equal(t, b, tb[i])
	}

}

func prepareTarFileSlice() []TarFile {

	defaultTestFileBody := "There is not a single vacant room throughout the entire infinite hotel."

	// Tree structure of the test file structure.
	// .
	// ├── file1
	// ├── file2
	// ├── path1dir1
	// │   ├── file1
	// │   ├── file2
	// │   ├── path1dir2
	// │   └── path1dir3
	// │       ├── file1
	// │       └── path1dir4
	// └── path2dir1
	//     ├── file1
	//     ├── file2
	//     ├── path2dir2
	//     │   └── file1
	//     └── path2dir3

	// This is an assembled default file structure to imitate an incoming tar stream
	filesToWrite := []TarFile{
		// level 1 directories
		{
			Name: path1dir1,
			Type: tar.TypeDir,
		},
		{
			Name: path2dir1,
			Type: tar.TypeDir,
		},
		// level 1 files
		{
			Name: l1file1,
			Type: tar.TypeReg,
			Body: defaultTestFileBody,
		},
		{
			Name: l1file2,
			Type: tar.TypeReg,
			Body: defaultTestFileBody,
		},

		// Level 2 directories
		{
			Name: path1dir2,
			Type: tar.TypeDir,
		},
		{
			Name: path1dir3,
			Type: tar.TypeDir,
		},
		{
			Name: path2dir2,
			Type: tar.TypeDir,
		},
		{
			Name: path2dir3,
			Type: tar.TypeDir,
		},

		// level 2 files
		{
			Name: l2file1,
			Type: tar.TypeReg,
			Body: defaultTestFileBody,
		},
		{
			Name: l2file2,
			Type: tar.TypeReg,
			Body: defaultTestFileBody,
		},
		{
			Name: l2file3,
			Type: tar.TypeReg,
			Body: defaultTestFileBody,
		},
		{
			Name: l2file4,
			Type: tar.TypeReg,
			Body: defaultTestFileBody,
		},

		// level 3 directories
		{
			Name: path1dir4,
			Type: tar.TypeDir,
		},

		// level 3 files
		{
			Name: l3file1,
			Type: tar.TypeReg,
			Body: defaultTestFileBody,
		},
		{
			Name: l3file2,
			Type: tar.TypeReg,
			Body: defaultTestFileBody,
		},
	}

	return filesToWrite
}

// Below are some structs and utility functions for unit testing the untar functionality

type TarFile struct {
	Name string
	Type byte
	Body string
}

func TarFiles(files []TarFile) (*bytes.Buffer, error) {
	// Create a buffer to write our archive to.
	buf := new(bytes.Buffer)

	// Create a new tar archive.
	tw := tar.NewWriter(buf)

	// Write data to the tar as if it came from the hub
	for _, file := range files {

		var hdr *tar.Header

		switch file.Type {
		case tar.TypeDir:
			hdr = &tar.Header{
				Name:     file.Name,
				Mode:     0777,
				Typeflag: file.Type,
				Size:     0,
			}
		case tar.TypeSymlink:
			hdr = &tar.Header{
				Name:     file.Name,
				Mode:     0777,
				Typeflag: file.Type,
				Size:     0,
				Linkname: file.Body,
			}
		default:
			hdr = &tar.Header{
				Name:     file.Name,
				Mode:     0777,
				Typeflag: file.Type,
				Size:     int64(len(file.Body)),
			}
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}

		if file.Type == tar.TypeDir || file.Type == tar.TypeSymlink {
			continue
		}

		if _, err := tw.Write([]byte(file.Body)); err != nil {
			return nil, err
		}
	}

	// Make sure to check the error on Close.
	if err := tw.Close(); err != nil {
		return nil, err
	}

	return buf, nil
}
