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

// Package vmomi is in a separate package to avoid the transitive inclusion of govmomi
// as a fundamental dependency of the main extraconfig
package vmomi

import (
	"fmt"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

// OptionValueMap returns a map from array of OptionValues
func OptionValueMap(src []types.BaseOptionValue) map[string]string {
	// create the key/value store from the extraconfig slice for lookups
	kv := make(map[string]string)
	for i := range src {
		k := src[i].GetOptionValue().Key
		v := src[i].GetOptionValue().Value.(string)
		kv[k] = UnescapeNil(v)
	}
	return kv
}

// OptionValueSource is a convenience method to generate a MapSource source from
// and array of OptionValue's
func OptionValueSource(src []types.BaseOptionValue) extraconfig.DataSource {
	kv := OptionValueMap(src)
	return extraconfig.MapSource(kv)
}

// OptionValueFromMap is a convenience method to convert a map into a BaseOptionValue array
// escapeNil - if true a nil string is replaced with "<nil>". Allows us to distinguish between
// deletion and nil as a value
func OptionValueFromMap(data map[string]string, escape bool) []types.BaseOptionValue {
	if len(data) == 0 {
		return nil
	}

	array := make([]types.BaseOptionValue, len(data))

	i := 0
	for k, v := range data {
		if escape {
			v = EscapeNil(v)
		}
		array[i] = &types.OptionValue{Key: k, Value: v}
		i++
	}

	return array
}

// OptionValueArrayToString translates the options array in to a Go formatted structure dump
func OptionValueArrayToString(options []types.BaseOptionValue) string {
	// create the key/value store from the extraconfig slice for lookups
	kv := make(map[string]string)
	for i := range options {
		k := options[i].GetOptionValue().Key
		v := options[i].GetOptionValue().Value.(string)
		kv[k] = v
	}

	return fmt.Sprintf("%#v", kv)
}

// OptionValueUpdatesFromMap generates an optionValue array for those entries in the map that do not
// already exist, are changed from the reference array, or a removed
// A removed entry will have a nil string for the value
// NOTE: DOES NOT CURRENTLY SUPPORT DELETION OF KEYS - KEYS MISSING FROM NEW MAP ARE IGNORED
func OptionValueUpdatesFromMap(existing []types.BaseOptionValue, new map[string]string) []types.BaseOptionValue {
	e := len(existing)
	if e == 0 {
		return OptionValueFromMap(new, true)
	}

	n := len(new)
	updates := make(map[string]string, n+e)
	unchanged := make(map[string]struct{}, n+e)

	// first the existing keys
	for i := range existing {
		v := existing[i].GetOptionValue()
		if nV, ok := new[v.Key]; ok && nV == v.Value.(string) {
			unchanged[v.Key] = struct{}{}
			// no change
			continue
		} else if ok {
			// changed
			updates[v.Key] = EscapeNil(nV)
		} else {
			// deletion
			// NOTE: ignored as this also deletes non VIC entries currently
			// there's no prefix for the non-guestinfo keys so cannot easily filter
			// updates[v.Key] = ""
		}
	}

	// now the new keys
	for k, v := range new {
		if _, ok := unchanged[k]; ok {
			continue
		}

		if _, ok := updates[k]; !ok {
			updates[k] = EscapeNil(v)
		}
	}

	return OptionValueFromMap(updates, false)
}

func EscapeNil(input string) string {
	if input == "" {
		return "<nil>"
	}

	return input
}

func UnescapeNil(input string) string {
	if input == "<nil>" {
		return ""
	}

	return input
}
