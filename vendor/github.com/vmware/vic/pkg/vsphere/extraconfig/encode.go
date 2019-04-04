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

package extraconfig

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrKeyNotFound = errors.New("key not found")
)

type encoder func(sink DataSink, src reflect.Value, prefix string, depth recursion)

var kindEncoders map[reflect.Kind]encoder
var intfEncoders map[reflect.Type]encoder

func init() {
	kindEncoders = map[reflect.Kind]encoder{
		reflect.String:  encodeString,
		reflect.Struct:  encodeStruct,
		reflect.Slice:   encodeSlice,
		reflect.Array:   encodeSlice,
		reflect.Map:     encodeMap,
		reflect.Ptr:     encodePtr,
		reflect.Int:     encodePrimitive,
		reflect.Int8:    encodePrimitive,
		reflect.Int16:   encodePrimitive,
		reflect.Int32:   encodePrimitive,
		reflect.Int64:   encodePrimitive,
		reflect.Bool:    encodePrimitive,
		reflect.Float32: encodePrimitive,
		reflect.Float64: encodePrimitive,
	}

	intfEncoders = map[reflect.Type]encoder{
		reflect.TypeOf(time.Time{}): encodeTime,
	}
}

// decode is the generic switcher that decides which decoder to use for a field
func encode(sink DataSink, src reflect.Value, prefix string, depth recursion) {
	// if depth has reached zero, we skip encoding entirely
	if depth.depth == 0 || depth.skipEncode {
		return
	}
	depth.depth--

	// obtain the handler from the map, checking for the more specific interfaces first
	enc, ok := intfEncoders[src.Type()]
	if ok {
		enc(sink, src, prefix, depth)
		return
	}

	enc, ok = kindEncoders[src.Kind()]
	if ok {
		enc(sink, src, prefix, depth)
		return
	}

	logger.Debugf("Skipping unsupported field, interface: %T, kind %s", src, src.Kind())
}

// encodeString is the degenerative case where what we get is what we need
func encodeString(sink DataSink, src reflect.Value, prefix string, depth recursion) {
	err := sink(prefix, src.String())
	if err != nil {
		logger.Errorf("Failed to encode string for key %s: %s", prefix, err)
	}

}

// encodePrimitive wraps the toString primitive encoding in a manner that can be called via encode
func encodePrimitive(sink DataSink, src reflect.Value, prefix string, depth recursion) {
	err := sink(prefix, toString(src))
	if err != nil {
		logger.Errorf("Failed to encode primitive for key %s: %s", prefix, err)
	}
}

func encodePtr(sink DataSink, src reflect.Value, prefix string, depth recursion) {
	// if we're not following pointers, return immediately
	if !depth.follow {
		return
	}

	logger.Debugf("Encoding object: %#v", src)

	if src.IsNil() {
		// no need to attempt anything
		return
	}

	encode(sink, src.Elem(), prefix, depth)
}

func encodeStruct(sink DataSink, src reflect.Value, prefix string, depth recursion) {
	logger.Debugf("Encoding object: %#v", src)

	// iterate through every field in the struct
	for i := 0; i < src.NumField(); i++ {
		field := src.Field(i)
		key, fdepth := calculateKeyFromField(src.Type().Field(i), prefix, depth)
		if key == "" {
			logger.Debugf("Skipping field %s with empty computed key", src.Type().Field(i).Name)
			continue
		}

		// Dump what we have so far
		logger.Debugf("Key: %s, Kind: %s Value: %s", key, field.Kind(), field.String())

		encode(sink, field, key, fdepth)
	}
}

func isEncodableSliceElemType(t reflect.Type) bool {
	switch t {
	case reflect.TypeOf((net.IP)(nil)):
		return true
	}

	return false
}

func encodeSlice(sink DataSink, src reflect.Value, prefix string, depth recursion) {
	logger.Debugf("Encoding object: %#v", src)

	length := src.Len()
	if length == 0 {
		logger.Debug("Skipping empty slice")
		return
	}

	// determine the key given the array type
	kind := src.Type().Elem().Kind()
	if kind == reflect.Uint8 {
		// special []byte array handling

		logger.Debugf("Converting []byte to base64 string")
		str := base64.StdEncoding.EncodeToString(src.Bytes())
		encode(sink, reflect.ValueOf(str), prefix, depth)
		return

	} else if kind == reflect.Struct || isEncodableSliceElemType(src.Type().Elem()) {
		for i := 0; i < length; i++ {
			// convert key to name|index format
			key := appendToPrefix(prefix, Separator, fmt.Sprintf("%d", i))
			encode(sink, src.Index(i), key, depth)
		}
	} else {
		// else assume it's primitive - we'll panic/recover and continue it not
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("unable to encode %s (slice) for %s: %s", src.Type(), prefix, err)
			}
		}()

		values := make([]string, length)
		for i := 0; i < length; i++ {
			values[i] = toString(src.Index(i))
		}

		// convert key to name|index format
		key := appendToPrefix(prefix, "", "~")
		err := sink(key, strings.Join(values, Separator))
		if err != nil {
			logger.Errorf("Failed to encode slice data for key %s: %s", key, err)
		}
	}

	// prefix contains the length of the array
	// seems insane calling toString(ValueOf(..)) but it means we're using the same path for everything
	err := sink(prefix, toString(reflect.ValueOf(length-1)))
	if err != nil {
		logger.Errorf("Failed to encode slice length for key %s: %s", prefix, err)
	}
}

func encodeMap(sink DataSink, src reflect.Value, prefix string, depth recursion) {
	logger.Debugf("Encoding object: %#v", src)

	// iterate over keys and recurse
	mkeys := src.MapKeys()
	length := len(mkeys)
	if length == 0 {
		logger.Debug("Skipping empty map")
		return
	}

	logger.Debugf("Encoding map entries based off prefix: %s", prefix)
	keys := make([]string, length)
	for i, v := range mkeys {
		keys[i] = toString(v)
		key := appendToPrefix(prefix, Separator, keys[i])
		encode(sink, src.MapIndex(v), key, depth)
	}

	// sort the keys before joining - purely to make testing viable
	sort.Strings(keys)
	err := sink(prefix, strings.Join(keys, Separator))
	if err != nil {
		logger.Errorf("Failed to encode map keys for key %s: %s", prefix, err)
	}

}

func encodeTime(sink DataSink, src reflect.Value, prefix string, depth recursion) {
	err := sink(prefix, src.Interface().(time.Time).String())
	if err != nil {
		logger.Errorf("Failed to encode time for key %s: %s", prefix, err)
	}

}

// toString converts a basic type to its string representation
func toString(field reflect.Value) string {
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(field.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(field.Uint(), 10)
	case reflect.Bool:
		return strconv.FormatBool(field.Bool())
	case reflect.String:
		return field.String()
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(field.Float(), 'E', -1, 64)
	default:
		panic(field.Type().String() + " is an unhandled type")
	}
}

// DataSink provides a function that, given a key/value will persist that
// in some manner suited for later retrieval
type DataSink func(string, string) error

// Encode serializes the given type to the supplied data sink
func Encode(sink DataSink, src interface{}) {
	encode(sink, reflect.ValueOf(src), "", Unbounded)
}

// EncodeWithPrefix serializes the given type to the supplied data sink, using
// the supplied prefix - this allows for serialization of subsections of a
// struct
func EncodeWithPrefix(sink DataSink, src interface{}, prefix string) {
	encode(sink, reflect.ValueOf(src), prefix, Unbounded)
}

// MapSink takes a map and populates it with key/value pairs from the encode
func MapSink(sink map[string]string) DataSink {
	// this is a very basic mechanism of allowing serialized updates to a sink
	// a more involved approach is necessary if wanting to do concurrent read/write
	mutex := sync.Mutex{}

	return func(key, value string) error {
		mutex.Lock()
		defer mutex.Unlock()

		sink[key] = value
		return nil
	}
}

// ScopeFilterSink will create a DataSink that only stores entries where the key scope
// matches one or more scopes in the filter.
// The filter is a bitwise composion of scope flags
func ScopeFilterSink(filter uint, sink DataSink) DataSink {
	return func(key, value string) error {
		logger.Debugf("Filtering encode of %s with scopes: %v", key, calculateScopeFromKey(key))
		scope := calculateScope(calculateScopeFromKey(key))
		if scope&filter != 0 {
			sink(key, value)
		} else {
			logger.Debugf("Skipping encode of %s with scopes that do not match filter: %v", key, calculateScopeFromKey(key))
		}
		return nil
	}
}
