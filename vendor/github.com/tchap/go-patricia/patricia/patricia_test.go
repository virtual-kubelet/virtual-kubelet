// Copyright (c) 2014 The go-patricia AUTHORS
//
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package patricia

import (
	"crypto/rand"
	"reflect"
	"testing"
)

// Tests -----------------------------------------------------------------------

func TestTrie_ConstructorOptions(t *testing.T) {
	trie := NewTrie(MaxPrefixPerNode(16), MaxChildrenPerSparseNode(10))

	if trie.maxPrefixPerNode != 16 {
		t.Errorf("Unexpected trie.maxPrefixPerNode value, expected=%v, got=%v",
			16, trie.maxPrefixPerNode)
	}

	if trie.maxChildrenPerSparseNode != 10 {
		t.Errorf("Unexpected trie.maxChildrenPerSparseNode value, expected=%v, got=%v",
			10, trie.maxChildrenPerSparseNode)
	}
}

func TestTrie_GetNonexistentPrefix(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"aba", 0, success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert(Prefix(v.key), v.value); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}

	t.Logf("GET prefix=baa, expect item=nil")
	if item := trie.Get(Prefix("baa")); item != nil {
		t.Errorf("Unexpected return value, expected=<nil>, got=%v", item)
	}
}

func TestTrie_RandomKitchenSink(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	const count, size = 750000, 16
	b := make([]byte, count+size+1)
	if _, err := rand.Read(b); err != nil {
		t.Fatal("error generating random bytes", err)
	}
	m := make(map[string]string)
	for i := 0; i < count; i++ {
		m[string(b[i:i+size])] = string(b[i+1 : i+size+1])
	}
	trie := NewTrie()
	getAndDelete := func(k, v string) {
		i := trie.Get(Prefix(k))
		if i == nil {
			t.Fatalf("item not found, prefix=%v", []byte(k))
		} else if s, ok := i.(string); !ok {
			t.Fatalf("unexpected item type, expecting=%v, got=%v", reflect.TypeOf(k), reflect.TypeOf(i))
		} else if s != v {
			t.Fatalf("unexpected item, expecting=%v, got=%v", []byte(k), []byte(s))
		} else if !trie.Delete(Prefix(k)) {
			t.Fatalf("delete failed, prefix=%v", []byte(k))
		} else if i = trie.Get(Prefix(k)); i != nil {
			t.Fatalf("unexpected item, expecting=<nil>, got=%v", i)
		} else if trie.Delete(Prefix(k)) {
			t.Fatalf("extra delete succeeded, prefix=%v", []byte(k))
		}
	}
	for k, v := range m {
		if !trie.Insert(Prefix(k), v) {
			t.Fatalf("insert failed, prefix=%v", []byte(k))
		}
		if byte(k[size/2]) < 128 {
			getAndDelete(k, v)
			delete(m, k)
		}
	}
	for k, v := range m {
		getAndDelete(k, v)
	}
}

// Make sure Delete that affects the root node works.
// This was panicking when Delete was broken.
func TestTrie_DeleteRoot(t *testing.T) {
	trie := NewTrie()

	v := testData{"aba", 0, success}

	t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
	if ok := trie.Insert(Prefix(v.key), v.value); ok != v.retVal {
		t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
	}

	t.Logf("DELETE prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
	if ok := trie.Delete(Prefix(v.key)); ok != v.retVal {
		t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
	}
}

func TestTrie_DeleteAbsentPrefix(t *testing.T) {
	trie := NewTrie()

	v := testData{"a", 0, success}

	t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
	if ok := trie.Insert(Prefix(v.key), v.value); ok != v.retVal {
		t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
	}

	d := "ab"
	t.Logf("DELETE prefix=%v, success=%v", d, failure)
	if ok := trie.Delete(Prefix(d)); ok != failure {
		t.Errorf("Unexpected return value, expected=%v, got=%v", failure, ok)
	}
	t.Logf("GET prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
	if i := trie.Get(Prefix(v.key)); i != v.value {
		t.Errorf("Unexpected item, expected=%v, got=%v", v.value, i)
	}
}
