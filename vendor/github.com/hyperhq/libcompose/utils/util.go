package utils

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"

	yaml "github.com/cloudfoundry-incubator/candiedyaml"
)

// InParallel holds a pool and a waitgroup to execute tasks in parallel and to be able
// to wait for completion of all tasks.
type InParallel struct {
	wg   sync.WaitGroup
	pool sync.Pool
}

// Add runs the specified task in parallel and adds it to the waitGroup.
func (i *InParallel) Add(task func() error) {
	i.wg.Add(1)

	go func() {
		defer i.wg.Done()
		err := task()
		if err != nil {
			i.pool.Put(err)
		}
	}()
}

// Wait waits for all tasks to complete and returns the latest error encountered if any.
func (i *InParallel) Wait() error {
	i.wg.Wait()
	obj := i.pool.Get()
	if err, ok := obj.(error); ok {
		return err
	}
	return nil
}

// ConvertByJSON converts a struct (src) to another one (target) using json marshalling/unmarshalling.
// If the structure are not compatible, this will throw an error as the unmarshalling will fail.
func ConvertByJSON(src, target interface{}) error {
	newBytes, err := json.Marshal(src)
	if err != nil {
		return err
	}

	err = json.Unmarshal(newBytes, target)
	if err != nil {
		logrus.Errorf("Failed to unmarshall: %v\n%s", err, string(newBytes))
	}
	return err
}

// Convert converts a struct (src) to another one (target) using yaml marshalling/unmarshalling.
// If the structure are not compatible, this will throw an error as the unmarshalling will fail.
func Convert(src, target interface{}) error {
	newBytes, err := yaml.Marshal(src)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(newBytes, target)
	if err != nil {
		logrus.Errorf("Failed to unmarshall: %v\n%s", err, string(newBytes))
	}
	return err
}

// CopySlice creates an exact copy of the provided string slice
func CopySlice(s []string) []string {
	if s == nil {
		return nil
	}
	r := make([]string, len(s))
	copy(r, s)
	return r
}

// CopyMap creates an exact copy of the provided string-to-string map
func CopyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	r := map[string]string{}
	for k, v := range m {
		r[k] = v
	}
	return r
}

// FilterStringSet accepts a string set `s` (in the form of `map[string]bool`) and a filtering function `f`
// and returns a string set containing only the strings `x` for which `f(x) == true`
func FilterStringSet(s map[string]bool, f func(x string) bool) map[string]bool {
	result := map[string]bool{}
	for k := range s {
		if f(k) {
			result[k] = true
		}
	}
	return result
}

// FilterString returns a json representation of the specified map
// that is used as filter for docker.
func FilterString(data map[string][]string) string {
	// I can't imagine this would ever fail
	bytes, _ := json.Marshal(data)
	return string(bytes)
}

// LabelFilterString returns a label json string representation of the specifed couple (key,value)
// that is used as filter for docker.
func LabelFilterString(key, value string) string {
	return FilterString(map[string][]string{
		"label": {fmt.Sprintf("%s=%s", key, value)},
	})
}

// LabelFilter returns a label map representation of the specifed couple (key,value)
// that is used as filter for docker.
func LabelFilter(key, value string) map[string][]string {
	return map[string][]string{
		"label": {fmt.Sprintf("%s=%s", key, value)},
	}
}

// Contains checks if the specified string (key) is present in the specified collection.
func Contains(collection []string, key string) bool {
	for _, value := range collection {
		if value == key {
			return true
		}
	}

	return false
}

// Merge performs a union of two string slices: the result is an unordered slice
// that includes every item from either argument exactly once
func Merge(coll1, coll2 []string) []string {
	m := map[string]struct{}{}
	for _, v := range append(coll1, coll2...) {
		m[v] = struct{}{}
	}
	r := make([]string, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}
