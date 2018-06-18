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

package extraconfig

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"

	"github.com/vmware/vic/pkg/log"
)

const (
	// DefaultTagName value
	DefaultTagName = "vic"
	// DefaultPrefix value
	DefaultPrefix = ""
	// DefaultGuestInfoPrefix value
	DefaultGuestInfoPrefix = "guestinfo.vice."
	//Separator for slice values and map keys
	Separator = "|"

	// suffix separator character
	suffixSeparator = "@"
	// secret suffix
	secretSuffix = "secret"
	// non-persistent suffix
	nonpersistentSuffix = "non-persistent"
)

const (
	// Invalid value
	Invalid = 1 << iota
	// Hidden value
	Hidden
	// ReadOnly value
	ReadOnly
	// ReadWrite value
	ReadWrite
	// NonPersistent value
	NonPersistent
	// Volatile value
	Volatile
	// Secret value
	Secret
)

type recursion struct {
	// depth is a recursion depth, 0 equating to skip field
	depth int
	// follow controls whether we follow pointers
	follow bool
}

// Unbounded is the value used for unbounded recursion
var Unbounded = recursion{depth: -1, follow: true}

var logger = &logrus.Logger{
	Out: os.Stderr,
	// We're using our own text formatter to skip the \n and \t escaping logrus
	// was doing on non TTY Out (we redirect to a file) descriptors.
	Formatter: log.NewTextFormatter(),
	Hooks:     make(logrus.LevelHooks),
	Level:     logrus.InfoLevel,
}

// SetLogLevel for the extraconfig package
func SetLogLevel(level logrus.Level) {
	logger.Level = level
}

// calculateScope returns the uint representation of scope tag
func calculateScope(scopes []string) uint {
	var scope uint

	empty := true
	for i := range scopes {
		if scopes[i] != "" {
			empty = false
			break
		}
	}

	if empty {
		return Hidden | ReadOnly
	}

	for _, v := range scopes {
		switch v {
		case "hidden":
			scope |= Hidden
		case "read-only":
			scope |= ReadOnly
		case "read-write":
			scope |= ReadWrite
		case nonpersistentSuffix:
			scope |= NonPersistent
		case "volatile":
			scope |= Volatile
		case secretSuffix:
			scope |= Secret | ReadOnly
		default:
			return Invalid
		}
	}
	return scope
}

func isSecret(key string) bool {
	suffix := strings.Split(key, suffixSeparator)
	if len(suffix) < 2 {
		// no @ separator
		return false
	}

	for i := range suffix[1:] {
		if suffix[i+1] == secretSuffix {
			return true
		}
	}

	return false
}

func isNonPersistent(key string) bool {
	suffix := strings.Split(key, suffixSeparator)
	if len(suffix) < 2 {
		// no @ separator
		return false
	}

	for i := range suffix[1:] {
		if suffix[i+1] == nonpersistentSuffix {
			return true
		}
	}

	return false
}

func calculateScopeFromKey(key string) []string {
	scopes := []string{}

	if !strings.HasPrefix(key, DefaultGuestInfoPrefix) {
		scopes = append(scopes, "hidden")
	}

	if strings.Contains(key, "/") {
		scopes = append(scopes, "read-only")
	} else {
		scopes = append(scopes, "read-write")
	}

	if isSecret(key) {
		scopes = append(scopes, secretSuffix)
	}

	if isNonPersistent(key) {
		scopes = append(scopes, nonpersistentSuffix)
	}

	return scopes
}

func calculateKeyFromField(field reflect.StructField, prefix string, depth recursion) (string, recursion) {
	skip := recursion{}
	//skip unexported fields
	if field.PkgPath != "" {
		logger.Debugf("Skipping %s (not exported)", field.Name)
		return "", skip
	}

	// get the annotations
	tags := field.Tag
	logger.Debugf("Tags: %#v", tags)

	var key string
	var scopes []string
	var scope uint

	fdepth := depth

	prefixScopes := calculateScopeFromKey(prefix)
	prefixScope := calculateScope(prefixScopes)

	// do we have DefaultTagName?
	if tags.Get(DefaultTagName) != "" {
		// get the scopes
		scopes = strings.Split(tags.Get("scope"), ",")
		logger.Debugf("Scopes: %#v", scopes)

		// get the keys and split properties from it
		key = tags.Get("key")
		logger.Debugf("Key specified: %s", key)

		// get the keys and split properties from it
		recurse := tags.Get("recurse")
		if recurse != "" {
			props := strings.Split(recurse, ",")
			// process properties
			for _, prop := range props {
				// determine recursion depth
				if strings.HasPrefix(prop, "depth") {
					parts := strings.Split(prop, "=")
					if len(parts) != 2 {
						logger.Warnf("Skipping field with incorrect recurse property: %s", prop)
						return "", skip
					}

					val, err := strconv.ParseInt(parts[1], 10, 64)
					if err != nil {
						logger.Warnf("Skipping field with incorrect recurse value: %s", parts[1])
						return "", skip
					}
					fdepth.depth = int(val)
				} else if prop == "nofollow" {
					fdepth.follow = false
				} else if prop == "follow" {
					fdepth.follow = true
				} else {
					logger.Warnf("Ignoring unknown recurse property %s (%s)", key, prop)
					continue
				}
			}
		}
	} else {
		logger.Debugf("%s not tagged - inheriting parent scope", field.Name)
		scopes = prefixScopes
	}

	if key == "" {
		logger.Debugf("%s does not specify key - defaulting to fieldname", field.Name)
		key = field.Name
	}

	scope = calculateScope(scopes)

	// non-persistent is inherited, even if other scopes are specified
	if prefixScope&NonPersistent != 0 {
		scope |= NonPersistent
	}

	// re-calculate the key based on the scope and prefix
	if key = calculateKey(scope, prefix, key); key == "" {
		logger.Debugf("Skipping %s (unknown scope %s)", field.Name, scopes)
		return "", skip
	}

	return key, fdepth
}

// calculateKey calculates the key based on the scope and current prefix
func calculateKey(scope uint, prefix string, key string) string {
	if scope&Invalid != 0 {
		logger.Debugf("invalid scope")
		return ""
	}

	newSep := "/"
	oldSep := "."
	key = strings.TrimSpace(key)

	hide := scope&Hidden != 0
	write := scope&ReadWrite != 0
	visible := strings.HasPrefix(prefix, DefaultGuestInfoPrefix)

	if !hide && write {
		oldSep = "/"
		newSep = "."
	}

	// strip any existing suffix from the prefix - it'll be re-added if still applicable
	suffix := strings.Index(prefix, suffixSeparator)
	if suffix != -1 {
		prefix = prefix[:suffix]
	}

	// assemble the actual keypath with appropriate separators
	out := key
	if prefix != "" {
		out = strings.Join([]string{prefix, key}, newSep)
	}

	if scope&Secret != 0 {
		out += suffixSeparator + secretSuffix
	}

	if scope&NonPersistent != 0 {
		if hide {
			logger.Debugf("Unable to combine non-persistent and hidden scopes")
			return ""
		}
		out += suffixSeparator + nonpersistentSuffix
	}

	// we don't care about existing separators when hiden
	if hide {
		if !visible {
			return out
		}

		// strip the prefix and the leading r/w signifier
		return out[len(DefaultGuestInfoPrefix)+1:]
	}

	// ensure that separators are correct
	out = strings.Replace(out, oldSep, newSep, -1)

	// Assemble the base that controls key publishing in guest
	if !visible {
		return DefaultGuestInfoPrefix + newSep + out
	}

	// prefix will have been mangled by strings.Replace
	return DefaultGuestInfoPrefix + out[len(DefaultGuestInfoPrefix):]
}

// utility function to allow adding of arbitrary prefix into key
// header is a leading segment that is preserved, prefix is injected after that
func addPrefixToKey(header, prefix, key string) string {
	if prefix == "" {
		return key
	}

	base := strings.TrimPrefix(key, header)
	separator := base[0]

	var modifiedPrefix string
	if separator == '.' {
		modifiedPrefix = strings.Replace(prefix, "/", ".", -1)
	} else {
		modifiedPrefix = strings.Replace(prefix, ".", "/", -1)
	}

	// we assume (given usage comment for WithPrefix) that there's no leading or trailing separator
	// on the prefix. base has a leading separator
	// guestinfoPrefix is const so adding it to the format string directly
	return fmt.Sprintf(header+"%c%s%s", separator, modifiedPrefix, base)
}

// appendToPrefix will join the value to the prefix with the separator (if any) while ensuring that
// any suffixes are moved to the end of the key
func appendToPrefix(prefix, separator, value string) string {
	// strip any existing suffix from the prefix - it'll be re-added if still applicable
	index := strings.Index(prefix, suffixSeparator)
	suffix := ""
	if index != -1 {
		suffix = prefix[index:]
		prefix = prefix[:index]
	}

	// suffix wil still include the suffix separator if present
	key := fmt.Sprintf("%s%s%s%s", prefix, separator, value, suffix)

	return key
}

func calculateKeys(v reflect.Value, field string, prefix string) []string {
	logger.Debugf("v=%#v, field=%#v, prefix=%#v", v, field, prefix)
	if v.Kind() == reflect.Ptr {
		return calculateKeys(v.Elem(), field, prefix)
	}

	if field == "" {
		return []string{prefix}
	}

	s := strings.SplitN(field, ".", 2)
	field = ""
	iterate := false
	if s[0] == "*" {
		iterate = true
	}

	if len(s) > 1 {
		field = s[1]
	}

	if !iterate {
		switch v.Kind() {
		case reflect.Map:
			found := false
			for _, k := range v.MapKeys() {
				sk := k.Convert(reflect.TypeOf(""))
				if sk.String() == s[0] {
					v = v.MapIndex(k)
					found = true
					break
				}
			}

			if !found {
				panic(fmt.Sprintf("could not find map key %s", s[0]))
			}
			prefix = appendToPrefix(prefix, Separator, s[0])
		case reflect.Array, reflect.Slice:
			i, err := strconv.Atoi(s[0])
			if err != nil {
				panic(fmt.Sprintf("bad array index %s: %s", s[0], err))
			}
			switch v.Type().Elem().Kind() {
			case reflect.Struct:
				prefix = appendToPrefix(prefix, Separator, fmt.Sprintf("%d", i))
			case reflect.Uint8:
				return []string{prefix}
			default:
				prefix = appendToPrefix(prefix, "", "~")
			}
			v = v.Index(i)
		case reflect.Struct:
			f, found := v.Type().FieldByName(s[0])
			if !found {
				panic(fmt.Sprintf("could not find field %s", s[0]))
			}
			prefix, _ = calculateKeyFromField(f, prefix, recursion{})
			v = v.FieldByIndex(f.Index)
		default:
			panic(fmt.Sprintf("cannot get field from type %s", v.Type()))
		}

		return calculateKeys(v, field, prefix)
	}

	var out []string
	switch v.Kind() {
	case reflect.Map:
		for _, k := range v.MapKeys() {
			sk := k.Convert(reflect.TypeOf(""))
			prefix := appendToPrefix(prefix, Separator, sk.String())
			out = append(out, calculateKeys(v.MapIndex(k), field, prefix)...)
		}
	case reflect.Array, reflect.Slice:
		switch v.Type().Elem().Kind() {
		case reflect.Struct:
			for i := 0; i < v.Len(); i++ {
				prefix := appendToPrefix(prefix, Separator, fmt.Sprintf("%d", i))
				out = append(out, calculateKeys(v.Index(i), field, prefix)...)
			}
		case reflect.Uint8:
			return []string{prefix}
		default:
			return []string{appendToPrefix(prefix, "", "~")}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			prefix, _ := calculateKeyFromField(v.Type().Field(i), prefix, recursion{})
			out = append(out, calculateKeys(v.Field(i), field, prefix)...)
		}
	default:
		panic(fmt.Sprintf("can't iterate type %s", v.Type().String()))
	}

	return out

}

// CalculateKeys gets the keys in extraconfig corresponding to the field
// specification passed in for obj. Examples:
//
//	  type struct A {
//	      I   int    `vic:"0.1" scope:"read-only" key:"i"`
//	      Str string `vic:"0.1" scope:"read-only" key:"str"`
//	  }
//
//	  type struct B {
//	      A A                      `vic:"0.1" scope:"read-only" key:"a"`
//	      Array []A                `vic:"0.1" scope:"read-only" key:"array"`
//	      Map   map[string]string  `vic:"0.1" scope:"read-only" key:"map"`
//	  }
//
//	  b := B{}
//	  b.Array = []A{A{}}
//	  b.Map = map[string]string{"foo": "", "bar": ""}
//	  // returns []string{"a/str"}
//	  CalculateKeys(b, "A.Str", "")
//
//	  // returns []string{"array|0"}
//	  CalculateKeys(b, "Array.0", "")
//
//	  // returns []string{"array|0"}
//	  CalculateKeys(b, "Array.*", "")
//
//	  // returns []string{"map|foo", "map|bar"}
//	  CalculateKeys(b, "Map.*", "")
//
//	  // returns []string{"map|foo"}
//	  CalculateKeys(b, "Map.foo", "")
//
//	  // returns []string{"map|foo/str"}
//	  CalculateKeys(b, "Map.foo.str", "")
//
func CalculateKeys(obj interface{}, field string, prefix string) []string {
	return calculateKeys(reflect.ValueOf(obj), field, prefix)
}
