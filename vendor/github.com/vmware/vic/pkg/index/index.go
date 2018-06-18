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
	"container/list"
	"errors"
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
)

var (
	ErrNodeNotFound = errors.New("Node not found")
)

type Element interface {
	// Returns the identifier of the node
	Self() string

	// Returns the string respresentation of the nodes parent (usually it's ID)
	Parent() string

	// Deep copy of the node
	Copy() Element
}

type node struct {
	Element
	parent   *node
	children []*node
	mask     uint32
}

func (n *node) addChild(child *node) {
	n.children = append(n.children, child)
}

type Index struct {
	root        *node
	lookupTable map[string]*node
	m           sync.RWMutex
}

func NewIndex() *Index {
	return &Index{
		lookupTable: make(map[string]*node),
	}
}

// Insert inserts a copy of the given node to the tree under the given parent.
func (i *Index) Insert(n Element) error {
	defer i.m.Unlock()
	i.m.Lock()

	_, ok := i.lookupTable[n.Self()]
	if ok {
		return fmt.Errorf("node %s already exists in index", n.Self())
	}

	log.Debugf("Index: inserting %s (parent: %s) in index", n.Self(), n.Parent())

	newNode := &node{
		Element: n.Copy(),
	}

	if n.Parent() == n.Self() {
		if i.root != nil {
			return fmt.Errorf("node cannot point to self unless it's root")
		}

		// set root
		i.root = newNode
	} else {
		p, ok := i.lookupTable[n.Parent()]
		if !ok {
			return fmt.Errorf("Can't find parent %s", n.Parent())
		}
		newNode.parent = p
		p.addChild(newNode)
	}

	i.lookupTable[n.Self()] = newNode
	return nil
}

// Get returns a Copy of the named node.
func (i *Index) Get(nodeID string) (Element, error) {
	defer i.m.RUnlock()
	i.m.RLock()

	n, ok := i.lookupTable[nodeID]
	if !ok {
		return nil, ErrNodeNotFound
	}

	return n.Copy(), nil
}

// HasChildren returns whether a node has children or not
func (i *Index) HasChildren(nodeID string) (bool, error) {
	defer i.m.RUnlock()
	i.m.RLock()

	n, ok := i.lookupTable[nodeID]
	if !ok {
		return false, ErrNodeNotFound
	}

	return (len(n.children) > 0), nil
}

func (i *Index) List() ([]Element, error) {
	defer i.m.RUnlock()
	i.m.RLock()

	nodes := make([]Element, 0, len(i.lookupTable))

	for _, v := range i.lookupTable {
		nodes = append(nodes, v.Copy())
	}

	return nodes, nil
}

// Delete deletes a leaf node
func (i *Index) Delete(nodeID string) (Element, error) {
	defer i.m.Unlock()
	i.m.Lock()

	return i.deleteNode(nodeID)
}

func (i *Index) deleteNode(nodeID string) (Element, error) {
	log.Debugf("deleting %s", nodeID)

	n, ok := i.lookupTable[nodeID]
	if !ok {
		return nil, fmt.Errorf("Node %s not found", nodeID)
	}

	if len(n.children) != 0 {
		return nil, fmt.Errorf("Node %s has children %#v", nodeID, n.children)
	}

	// remove the reference to the node from its parent
	parent := n.parent
	var deleted bool
	for idx, child := range parent.children {
		if child.Self() == nodeID {
			parent.children = append(parent.children[:idx], parent.children[idx+1:]...)
			deleted = true
		}
	}

	if !deleted {
		err := fmt.Errorf("%s not found in tree", nodeID)
		log.Errorf("%s", err)
		return nil, err
	}

	// remove from the lookup table
	delete(i.lookupTable, nodeID)
	n.parent = nil

	return n.Element, nil
}

type iterflag int

const (
	NOOP iterflag = iota
	STOP
)

type visitor func(Element) (iterflag, error)

func (i *Index) bfs(root *node, visitFunc visitor) error {
	defer i.m.Unlock()
	i.m.Lock()

	// XXX Look into parallelizing this without breaking API boundaries.
	return i.bfsworker(root, func(n *node) (iterflag, error) { return visitFunc(n.Element) })
}

func (i *Index) bfsworker(root *node, visitFunc func(*node) (iterflag, error)) error {

	queue := list.New()
	queue.PushBack(root)

	for queue.Len() > 0 {
		n := queue.Remove(queue.Front()).(*node)

		flag, err := visitFunc(n)
		if err != nil {
			return err
		}

		if flag == STOP {
			return nil
		}

		for _, child := range n.children {
			queue.PushBack(child)
		}
	}

	return nil
}
