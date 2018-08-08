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

package datastore

import (
	"context"
	"net/url"
	"os"
	"path"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/vsphere/datastore/test"
	"github.com/vmware/vic/pkg/vsphere/tasks"
)

func dsSetup(t *testing.T) (context.Context, *Helper, func()) {
	log.SetLevel(log.DebugLevel)
	ctx := context.Background()
	sess := test.Session(ctx, t)

	ds, err := NewHelper(ctx, sess, sess.Datastore, TestName("dstests"))
	if !assert.NoError(t, err) {
		return ctx, nil, nil
	}

	f := func() {
		log.Infof("Removing test root %s", ds.RootURL.String())
		err := tasks.Wait(ctx, func(context.Context) (tasks.Task, error) {
			return ds.fm.DeleteDatastoreFile(ctx, ds.RootURL.String(), sess.Datacenter)
		})

		if err != nil {
			log.Errorf(err.Error())
			return
		}
	}

	return ctx, ds, f
}

// test if we can get a Datastore via the rooturl
func TestDatastoreGetDatastores(t *testing.T) {
	ctx, ds, cleanupfunc := dsSetup(t)
	if t.Failed() {
		return
	}
	defer cleanupfunc()

	firstSummary, err := ds.Summary(ctx)
	if !assert.NoError(t, err) {
		return
	}

	t.Logf("Name:\t%s\n", firstSummary.Name)
	t.Logf("  Path:\t%s\n", ds.ds.InventoryPath)
	t.Logf("  Type:\t%s\n", firstSummary.Type)
	t.Logf("  URL:\t%s\n", firstSummary.Url)
	t.Logf("  Capacity:\t%.1f GB\n", float64(firstSummary.Capacity)/(1<<30))
	t.Logf("  Free:\t%.1f GB\n", float64(firstSummary.FreeSpace)/(1<<30))

	inMap := make(map[string]*url.URL)

	p, err := url.Parse(ds.RootURL.String())
	if !assert.NoError(t, err) {
		return
	}

	inMap["foo"] = p

	dstores, err := GetDatastores(context.TODO(), ds.s, inMap)
	if !assert.NoError(t, err) || !assert.NotNil(t, dstores) {
		return
	}

	secondSummary, err := ds.Summary(ctx)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Equal(t, firstSummary, secondSummary) {
		return
	}
}

func TestDatastoreRestart(t *testing.T) {
	// creates a root in the datastore
	ctx, ds, cleanupfunc := dsSetup(t)
	if t.Failed() {
		return
	}
	defer cleanupfunc()

	// Create a nested dir in the root and use that as the datastore
	nestedRoot := path.Join(ds.RootURL.Path, "foo")
	ds, err := NewHelper(ctx, ds.s, ds.s.Datastore, nestedRoot)
	if !assert.NoError(t, err) {
		return
	}

	// test we can ls the root
	_, err = ds.Ls(ctx, "")
	if !assert.NoError(t, err) {
		return
	}

	// create a dir
	_, err = ds.Mkdir(ctx, true, "baz")
	if !assert.NoError(t, err) {
		return
	}

	// create a new datastore object with the same path as the nested one
	ds, err = NewHelper(ctx, ds.s, ds.s.Datastore, nestedRoot)
	if !assert.NoError(t, err) {
		return
	}

	// try to create the same baz dir, assert it exists
	_, err = ds.Mkdir(ctx, true, "baz")
	if !assert.Error(t, err) || !assert.True(t, os.IsExist(err)) {
		return
	}

	assert.NotEmpty(t, ds.RootURL)
}

func TestDatastoreCreateDir(t *testing.T) {
	ctx, ds, cleanupfunc := dsSetup(t)
	if t.Failed() {
		return
	}
	defer cleanupfunc()

	_, err := ds.Ls(ctx, "")
	if !assert.NoError(t, err) {
		return
	}

	// assert create dir of a dir that exists is os.ErrExists
	_, err = ds.Mkdir(ctx, true, "foo")
	if !assert.NoError(t, err) {
		return
	}

	_, err = ds.Mkdir(ctx, true, "foo")
	if !assert.Error(t, err) || !assert.True(t, os.IsExist(err)) {
		return
	}
}

func TestDatastoreMkdirAndLs(t *testing.T) {
	ctx, ds, cleanupfunc := dsSetup(t)
	if t.Failed() {
		return
	}
	defer cleanupfunc()

	dirs := []string{"dir1", "dir1/child1"}

	// create the dir then test it exists by calling ls
	for _, dir := range dirs {
		_, err := ds.Mkdir(ctx, true, dir)
		if !assert.NoError(t, err) {
			return
		}

		_, err = ds.Ls(ctx, dir)
		if !assert.NoError(t, err) {
			return
		}
	}
}

func TestDatastoreToURLParsing(t *testing.T) {
	expectedURL := "ds://datastore1/path/to/thing"

	input := [][]string{
		{"[datastore1] /path/to/thing", expectedURL},
		{"[datastore1] path/to/thing", expectedURL},
		{"[datastore1] ///path////to/thing", expectedURL},
		{"[Datastore (1)] /path/to/thing", "ds://Datastore%20(1)/path/to/thing"},
		{"[datastore1] path", "ds://datastore1/path"},
		{"[datastore1] pa-th", "ds://datastore1/pa-th"},
		{"[datastore1] pa_th", "ds://datastore1/pa_th"},
		{"[data_store1] pa_th", "ds://data_store1/pa_th"},
	}

	dsoutputs := []string{
		"[datastore1] /path/to/thing",
		"[datastore1] path/to/thing",
		"[datastore1] /path/to/thing",
		"[Datastore (1)] /path/to/thing",
		"[datastore1] path",
		"[datastore1] pa-th",
		"[datastore1] pa_th",
		"[data_store1] pa_th",
	}

	for i, in := range input {
		u, err := ToURL(in[0])

		if !assert.NoError(t, err) || !assert.NotNil(t, u) {
			return
		}

		if !assert.Equal(t, in[1], u.String()) {
			return
		}

		out, err := URLtoDatastore(u)
		if !assert.NoError(t, err) || !assert.True(t, len(out) > 0) {
			return
		}

		if !assert.Equal(t, dsoutputs[i], out) {
			return
		}
	}
}
