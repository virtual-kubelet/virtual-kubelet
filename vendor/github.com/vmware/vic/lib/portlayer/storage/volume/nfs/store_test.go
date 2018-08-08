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
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/trace"
)

const (
	nfsTestDir = "NFSVolumeStoreTests"
)

type MockMount struct {
	Path string
}

func (m MockMount) Mount(op trace.Operation) (Target, error) {
	return NewMocktarget(m.Path), nil
}

func (m MockMount) Unmount(op trace.Operation) error {
	return nil
}

func (m MockMount) URL() (*url.URL, error) {
	return url.Parse("nfs://localhost/some/interesting/dir")
}

type MockTarget struct {
	dirPath string
}

func NewMocktarget(pth string) MockTarget {
	return MockTarget{dirPath: pth}
}

func (v MockTarget) Open(pth string) (io.ReadCloser, error) {
	pth = path.Join(v.dirPath, pth)
	logrus.Infof("open(%s)", pth)
	return os.Open(pth)
}

func (v MockTarget) OpenFile(pth string, mode os.FileMode) (io.ReadWriteCloser, error) {
	pth = path.Join(v.dirPath, pth)
	logrus.Infof("openfile(%s)", pth)
	return os.OpenFile(pth, os.O_RDWR|os.O_CREATE, mode)
}

func (v MockTarget) Mkdir(pth string, perm os.FileMode) ([]byte, error) {
	pth = path.Join(v.dirPath, pth)
	logrus.Infof("mkdir(%s)", pth)
	return nil, os.Mkdir(pth, perm)
}

func (v MockTarget) RemoveAll(pth string) error {
	pth = path.Join(v.dirPath, pth)
	logrus.Infof("RemoveAll(%s)", pth)
	return os.RemoveAll(pth)
}

func (v MockTarget) ReadDir(pth string) ([]os.FileInfo, error) {
	pth = path.Join(v.dirPath, pth)
	logrus.Infof("readdir(%s)", pth)
	dir, err := os.Open(pth)
	defer dir.Close()
	if err != nil {
		return nil, err
	}

	return dir.Readdir(0)
}

func (v MockTarget) Lookup(pth string) (os.FileInfo, []byte, error) {
	pth = path.Join(v.dirPath, pth)
	logrus.Infof("stat(%s)", pth)
	info, err := os.Stat(pth)
	return info, nil, err
}

var (
	expected Target
	mnt      MountServer
)

func TestMain(m *testing.M) {

	nfsurl := os.Getenv("NFS_TEST_URL")
	if nfsurl != "" {
		u, err := url.Parse(nfsurl)
		if err != nil {
			logrus.Errorf(err.Error())
			os.Exit(-1)
		}

		logrus.Infof("testing nfs against %#v", u)

		mnt = NewMount(u, "hasselhoff", 1001, 10001)
		expected, err = mnt.Mount(trace.NewOperation(context.TODO(), "mount"))
		if err != nil {
			logrus.Errorf("error mounting %s: %s", u.String(), err.Error())
			os.Exit(-1)
		}

	} else {

		testdir, err := ioutil.TempDir(os.TempDir(), nfsTestDir)
		if err != nil {
			logrus.Errorf("error creating tmpdir: %s", err.Error())
			os.Exit(-1)
		}

		os.Mkdir(testdir, 0755)
		defer os.RemoveAll(testdir)

		// We can twiddle the target directly via expected and use it to verify the
		// right things are happening.
		mnt = &MockMount{testdir}
		expected = &MockTarget{testdir}
	}

	result := m.Run()
	os.Exit(result)
}

func TestSimpleVolumeStoreOperations(t *testing.T) {
	op := trace.NewOperation(context.TODO(), "TestOp")

	// Create a Volume Store
	vs, err := NewVolumeStore(op, "testStore", mnt)
	if !assert.NoError(t, err, "Failed during call to NewVolumeStore with err (%s)", err) {
		return
	}

	_, _, err = expected.Lookup(volumesDir)
	if !assert.NoError(t, err, "Could not find the initial volume store directory after creation of volume store. err (%s)", err) {
		return
	}

	if !assert.NotNil(t, vs, "Volume Store created with nil err, but return is also nil") {
		return
	}

	info := make(map[string][]byte)
	testInfoKey := "junk"
	info[testInfoKey] = make([]byte, 20)

	// Create a Volume
	testVolName := "testVolume"

	vol, err := vs.VolumeCreate(op, testVolName, vs.SelfLink, 0 /*we do not use this*/, info)
	if !assert.NoError(t, err, "Failed during call to VolumeCreate with err (%s)", err) {
		return
	}

	if !assert.Equal(t, testVolName, vol.ID, "expected volume ID (%s) got ID (%s)", testVolName, vol.ID) {
		return
	}

	_, ok := vol.Info[testInfoKey]
	if !assert.True(t, ok, "TestInfoKey did not exist in the return metadata map") {
		return
	}

	// Check Metadata Pathing
	metaDirEntries, err := expected.ReadDir(metadataDir)
	if !assert.NoError(t, err, "Failed to read the metadata directory with err (%s)", err) {
		return
	}

	if !assert.Len(t, metaDirEntries, 1) {
		return
	}

	volumeDirEntries, err := expected.ReadDir(volumesDir)
	if !assert.NoError(t, err, "Failed to read the volume data directory with err (%s)", err) {
		return
	}

	if !assert.Len(t, volumeDirEntries, 1) {
		return
	}

	// Remove the Volume
	err = vs.VolumeDestroy(op, vol)
	if !assert.NoError(t, err, "Failed during a call to VolumeDestroy with err (%s)", err) {
		return
	}

	// should throw an error since the directory got nuked
	metaDirEntries, err = expected.ReadDir(path.Join(metadataDir, vol.ID))
	if !assert.Error(t, err) {
		return
	}

	if !assert.Equal(t, len(metaDirEntries), 0, "expected metadata directory to have 1 entry and it had (%s)", len(metaDirEntries)) {
		return
	}

	// Should throw an error on the volume directory
	volumeDirEntries, err = expected.ReadDir(path.Join(volumesDir, vol.ID))
	if !assert.Error(t, err) {
		return
	}

	if !assert.Equal(t, len(volumeDirEntries), 0, "expected metadata directory to have 1 entry and it had (%s)", len(volumeDirEntries)) {
		return
	}

	volToCheck, err := vs.VolumeCreate(op, testVolName, vs.SelfLink, 0, info)
	if !assert.NoError(t, err, "Failed during call to VolumeCreate with err (%s)", err) {
		return
	}

	volumeList, err := vs.VolumesList(op)
	if !assert.NoError(t, err, "Failed during call to VolumesList with err (%s)", err) {
		return
	}

	if !assert.Equal(t, 1, len(volumeList)) {
		return
	}

	if !assert.Equal(t, volumeList[0].ID, volToCheck.ID, "Failed due to VolumeList returning an unexpected volume %#v when volume %#v was expected.", volumeList[0], volToCheck) {
		return
	}

	RetrievedInfo := volumeList[0].Info
	CreatedInfo := volToCheck.Info

	if !assert.Equal(t, len(RetrievedInfo), len(CreatedInfo), "Length mismatch between the created volume(%s) and the volume returned from VolumeList(%s)", len(CreatedInfo), len(RetrievedInfo)) {
		return
	}

	if !assert.Equal(t, RetrievedInfo[testInfoKey], CreatedInfo[testInfoKey], "Failed due to mismatch in metadata between the content of the Created volume(%s) and the volume return from VolumesList", CreatedInfo[testInfoKey], RetrievedInfo[testInfoKey]) {
		return
	}

	err = vs.VolumeDestroy(op, volToCheck)
	if !assert.NoError(t, err, "Failed during a call to VolumeDestroy with err (%s)", err) {
		return
	}

	volumeList, err = vs.VolumesList(op)
	if !assert.NoError(t, err, "Failed during a call to VolumesListwith err (%s)", err) {
		return
	}

	if !assert.Equal(t, len(volumeList), 0, "Expected %s volumes, VolumesList returned %s", 0, len(volumeList)) {
		return
	}
}

func TestMultipleVolumes(t *testing.T) {
	op := trace.NewOperation(context.TODO(), "TestOp")

	//Create a Volume Store
	vs, err := NewVolumeStore(op, "testStore", mnt)
	if !assert.NoError(t, err, "Failed during call to NewVolumeStore with err (%s)", err) {
		return
	}

	if !assert.NotNil(t, vs, "Volume Store created with nil err, but return is also nil") {
		return
	}

	_, _, err = expected.Lookup(volumesDir)
	if !assert.NoError(t, err, "Could not find the initial volume store directory after creation of volume store. err (%s)", err) {
		return
	}

	// setup volume inputs
	testVolNameOne := "test1"
	infoOne := make(map[string][]byte)
	testOneInfoKey := "junk"
	infoOne[testOneInfoKey] = make([]byte, 20)

	testVolNameTwo := "test2"
	testTwoInfoKey := "important"
	infoTwo := make(map[string][]byte)
	infoTwo[testTwoInfoKey] = []byte("42")
	testTwoInfoKeyTwo := "lessImportant"
	infoTwo[testTwoInfoKeyTwo] = []byte("41")

	testVolNameThree := "test3"
	infoThree := make(map[string][]byte)
	testThreeInfoKey := "lotsOfStuff"
	infoThree[testThreeInfoKey] = []byte("importantData")
	testThreeInfoKeyTwo := "someMoreStuff"
	infoThree[testThreeInfoKeyTwo] = []byte("maybeSomeLabels")

	//make volume one
	volOne, err := vs.VolumeCreate(op, testVolNameOne, vs.SelfLink, 0 /*we do not use this*/, infoOne)

	if !assert.NoError(t, err, "Failed during call to VolumeCreate with err (%s)", err) {
		return
	}

	if !assert.Equal(t, testVolNameOne, volOne.ID, "expected volume ID (%s) got ID (%s)", testVolNameOne, volOne.ID) {
		return
	}

	valOne, ok := volOne.Info[testOneInfoKey]
	if !assert.True(t, ok, "TestInfoKey did not exist in the return metadata map") {
		return
	}

	if !assert.Equal(t, valOne, volOne.Info[testOneInfoKey], "TestVolOne expected to have data (%s) and (%s) was found", infoOne, valOne) {
		return
	}

	// make volume two
	volTwo, err := vs.VolumeCreate(op, testVolNameTwo, vs.SelfLink, 0 /*we do not use this*/, infoTwo)

	if !assert.NoError(t, err, "Failed during call to VolumeCreate with err (%s)", err) {
		return
	}

	if !assert.Equal(t, testVolNameTwo, volTwo.ID, "expected volume ID (%s) got ID (%s)", testVolNameTwo, volTwo.ID) {
		return
	}

	valOne, ok = volTwo.Info[testTwoInfoKey]
	if !assert.True(t, ok, "TestInfoKey did not exist in the return metadata map") {
		return
	}

	if !assert.Equal(t, []byte("42"), valOne, "TestVolTwo expected to have data (%s) and (%s) was found", []byte("42"), valOne) {
		return
	}

	valTwo, ok := volTwo.Info[testTwoInfoKeyTwo]
	if !assert.True(t, ok, "TestTwoInfoKeyTwo did not exist in the return metadata map for volTwo") {
		return
	}

	if !assert.Equal(t, []byte("41"), valTwo, "TestVolTwo expected to have data (%s) and (%s) was found", []byte("41"), valTwo) {
		return
	}

	// make volume three
	volThree, err := vs.VolumeCreate(op, testVolNameThree, vs.SelfLink, 0 /*we do not use this*/, infoThree)

	if !assert.NoError(t, err, "Failed during call to VolumeCreate with err (%s)", err) {
		return
	}

	if !assert.Equal(t, testVolNameThree, volThree.ID, "expected volume ID (%s) got ID (%s)", testVolNameThree, volThree.ID) {
		return
	}

	valOne, ok = volThree.Info[testThreeInfoKey]
	if !assert.True(t, ok, "TestInfoKey did not exist in the return metadata map") {
		return
	}

	if !assert.Equal(t, []byte("importantData"), valOne, "TestVolThree expected to have data (%s) and (%s) was found", []byte("importantData"), valOne) {
		return
	}

	valTwo, ok = volThree.Info[testThreeInfoKeyTwo]
	if !assert.True(t, ok, "TestThreeInfoKeyTwo did not exist in the return metadata map for volThree") {
		return
	}

	if !assert.Equal(t, []byte("maybeSomeLabels"), valTwo, "TestVolThree expected to have data (%s) and (%s) was found", []byte("maybeSomeLabels"), valTwo) {
		return
	}

	// list volumes
	volumes, err := vs.VolumesList(op)
	if !assert.NoError(t, err, "Failed during a call to VolumesList with err (%s)", err) {
		return
	}

	volCount := len(volumes)
	if !assert.Equal(t, volCount, 3, "VolumesList returned unexpected volume count. expected (%s), but received (%s) ", 3, volCount) {
		return
	}

	// check metadatas
	metaDirEntries, err := expected.ReadDir(metadataDir)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Equal(t, len(metaDirEntries), 3, "expected metadata directory to have 1 entry and it had (%s)", len(metaDirEntries)) {
		return
	}

	volumeDirEntries, err := expected.ReadDir(volumesDir)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Equal(t, len(volumeDirEntries), 3, "expected metadata directory to have 1 entry and it had (%s)", len(volumeDirEntries)) {
		return
	}

	verify := func(vols map[string]int) error {
		for name, num := range vols {
			metadataFiles, err := expected.ReadDir(path.Join(metadataDir, name))
			if err != nil {
				return err
			}

			if len(metadataFiles) != num {
				return fmt.Errorf("len metadata %d != %d", len(metadataFiles), num)
			}
		}

		return nil
	}

	// check and individual metadata dir
	volmap := map[string]int{
		testVolNameThree: 2,
		testVolNameTwo:   2,
		testVolNameOne:   1,
	}
	if !assert.NoError(t, verify(volmap)) {
		return
	}

	// remove volume one
	err = vs.VolumeDestroy(op, volOne)
	if !assert.NoError(t, err, "Failed during a call to VolumeDestroy with error (%s)", err) {
		return
	}

	// assert it's gone
	_, _, err = expected.Lookup(path.Join(metadataDir, volOne.ID))
	if !assert.Error(t, err) {
		return
	}

	_, _, err = expected.Lookup(path.Join(volumesDir, volOne.ID))
	if !assert.Error(t, err) {
		return
	}

	// check that volume two and three exist with appropriate metadata
	delete(volmap, testVolNameOne)
	if !assert.NoError(t, verify(volmap)) {
		return
	}

	// remove the rest of the volumes
	err = vs.VolumeDestroy(op, volTwo)
	if !assert.NoError(t, err, "Failed during a call to VolumeDestroy with error (%s)", err) {
		return
	}

	err = vs.VolumeDestroy(op, volThree)
	if !assert.NoError(t, err, "Failed during a call to VolumeDestroy with error (%s)", err) {
		return
	}

	// verify they're gone
	vols, err := vs.VolumesList(op)
	if !assert.Len(t, vols, 0) {
		return
	}

	return
}
