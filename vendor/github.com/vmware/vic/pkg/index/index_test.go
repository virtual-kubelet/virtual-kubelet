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

package index

import (
	"strconv"
	"sync"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type mockEntry struct {
	number int
	parent int
}

func newMockEntry(n, parent int) *mockEntry {
	return &mockEntry{
		number: n,
		parent: parent,
	}
}

func (m *mockEntry) Self() string {
	return strconv.Itoa(m.number)
}

func (m *mockEntry) Parent() string {
	return strconv.Itoa(m.parent)
}

func (m *mockEntry) Copy() Element {
	return &mockEntry{
		number: m.number,
		parent: m.parent,
	}
}

func TestInsertAndGet(t *testing.T) {
	i := NewIndex()
	root := newMockEntry(0, 0)
	err := i.Insert(root)
	if !assert.NoError(t, err) {
		return
	}

	max := 10

	// insert
	for n := 1; n < max; n++ {
		err = i.Insert(newMockEntry(n, n-1))
		if !assert.NoError(t, err) {
			return
		}
	}

	// add an entry that already exists
	err = i.Insert(newMockEntry(1, 1))
	if !assert.Error(t, err) {
		return
	}

	// check children
	for _, node := range i.lookupTable {
		if node.Self() != "9" && !assert.True(t, len(node.children) > 0) {
			return
		}
	}

	// Now get
	wg := sync.WaitGroup{}
	wg.Add(max)
	for idx := 0; idx < max; idx++ {
		go func(idx int) {
			defer wg.Done()
			_, err := i.Get(strconv.Itoa(idx))
			if !assert.NoError(t, err) {
				return
			}
		}(idx)
	}
	wg.Wait()
}

func TestBFS(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	i := NewIndex()

	root := newMockEntry(0, 0)

	i.Insert(root)

	branches := 10
	expectedNodes := createTree(t, i, branches)
	expectedNodes[0] = root

	var count int
	err := i.bfs(i.root, func(n Element) (iterflag, error) {
		mynode := n.(*mockEntry)
		t.Logf("%#v\n", mynode)

		// will point to different elements but check their values
		assert.Equal(t, expectedNodes[mynode.number], mynode)
		count++
		return NOOP, nil
	})

	if !assert.NoError(t, err) {
		return
	}

	if !assert.Equal(t, (4*(branches-1))+1, count) {
		return
	}
}

func TestDelete(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	i := NewIndex()

	root := newMockEntry(0, 0)

	i.Insert(root)

	branches := 10
	expectedNodes := createTree(t, i, branches)
	expectedNodes[0] = root

	// list what we added
	before, err := i.List()
	if !assert.NoError(t, err) || !assert.True(t, len(before) > 0) {
		return
	}

	// is a leaf, should delete without issue
	n, err := i.Delete("9000")
	if !assert.NoError(t, err) || !assert.NotNil(t, n) {
		return
	}

	// check it's gone
	_, ok := i.lookupTable["9000"]
	if !assert.False(t, ok) {
		return
	}

	// isn't a leaf, should throw an error
	n, err = i.Delete("9")
	if !assert.Error(t, err) || !assert.Nil(t, n) {
		return
	}

	// isn't in the index
	n, err = i.Delete("foo")
	if !assert.Error(t, err) || !assert.Nil(t, n) {
		return
	}

	// list once more and make sure we nuked the image
	after, err := i.List()
	if !assert.NoError(t, err) || !assert.Equal(t, len(before), len(after)+1) {
		return
	}
}

func createTree(t *testing.T, i *Index, count int) map[int]*mockEntry {
	expectedNodes := make(map[int]*mockEntry)

	insert := func(n *mockEntry) {
		if !assert.NoError(t, i.Insert(n)) {
			return
		}
	}

	// insert and create 3 children for each branch
	for n := 1; n < count; n++ {
		expectedNodes[n] = newMockEntry(n, 0)
		expectedNodes[n*10] = newMockEntry(n*10, n)
		expectedNodes[n*100] = newMockEntry(n*100, n)
		expectedNodes[n*1000] = newMockEntry(n*1000, n)

		insert(expectedNodes[n])
		insert(expectedNodes[n*10])
		insert(expectedNodes[n*100])
		insert(expectedNodes[n*1000])
	}

	return expectedNodes
}
