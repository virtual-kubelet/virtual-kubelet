package yaml

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/flynn/go-shlex"
)

// Stringorslice represents a string or an array of strings.
// TODO use docker/docker/pkg/stringutils.StrSlice once 1.9.x is released.
type Stringorslice struct {
	parts []string
}

// MarshalYAML implements the Marshaller interface.
func (s Stringorslice) MarshalYAML() (tag string, value interface{}, err error) {
	return "", s.parts, nil
}

func toStrings(s []interface{}) ([]string, error) {
	if len(s) == 0 {
		return nil, nil
	}
	r := make([]string, len(s))
	for k, v := range s {
		if sv, ok := v.(string); ok {
			r[k] = sv
		} else {
			return nil, fmt.Errorf("Cannot unmarshal '%v' of type %T into a string value", v, v)
		}
	}
	return r, nil
}

// UnmarshalYAML implements the Unmarshaller interface.
func (s *Stringorslice) UnmarshalYAML(tag string, value interface{}) error {
	switch value := value.(type) {
	case []interface{}:
		parts, err := toStrings(value)
		if err != nil {
			return err
		}
		s.parts = parts
	case string:
		s.parts = []string{value}
	default:
		return fmt.Errorf("Failed to unmarshal Stringorslice: %#v", value)
	}
	return nil
}

// Len returns the number of parts of the Stringorslice.
func (s *Stringorslice) Len() int {
	if s == nil {
		return 0
	}
	return len(s.parts)
}

// Slice gets the parts of the StrSlice as a Slice of string.
func (s *Stringorslice) Slice() []string {
	if s == nil {
		return nil
	}
	return s.parts
}

// NewStringorslice creates an Stringorslice based on the specified parts (as strings).
func NewStringorslice(parts ...string) Stringorslice {
	return Stringorslice{parts}
}

// Ulimits represents a list of Ulimit.
// It is, however, represented in yaml as keys (and thus map in Go)
type Ulimits struct {
	Elements []Ulimit
}

// MarshalYAML implements the Marshaller interface.
func (u Ulimits) MarshalYAML() (tag string, value interface{}, err error) {
	ulimitMap := make(map[string]Ulimit)
	for _, ulimit := range u.Elements {
		ulimitMap[ulimit.Name] = ulimit
	}
	return "", ulimitMap, nil
}

// UnmarshalYAML implements the Unmarshaller interface.
func (u *Ulimits) UnmarshalYAML(tag string, value interface{}) error {
	ulimits := make(map[string]Ulimit)
	yamlUlimits := reflect.ValueOf(value)
	switch yamlUlimits.Kind() {
	case reflect.Map:
		for _, key := range yamlUlimits.MapKeys() {
			var name string
			var soft, hard int64
			mapValue := yamlUlimits.MapIndex(key).Elem()
			name = key.Elem().String()
			switch mapValue.Kind() {
			case reflect.Int64:
				soft = mapValue.Int()
				hard = mapValue.Int()
			case reflect.Map:
				if len(mapValue.MapKeys()) != 2 {
					return fmt.Errorf("Failed to unmarshal Ulimit: %#v", mapValue)
				}
				for _, subKey := range mapValue.MapKeys() {
					subValue := mapValue.MapIndex(subKey).Elem()
					switch subKey.Elem().String() {
					case "soft":
						soft = subValue.Int()
					case "hard":
						hard = subValue.Int()
					}
				}
			default:
				return fmt.Errorf("Failed to unmarshal Ulimit: %#v, %v", mapValue, mapValue.Kind())
			}
			ulimits[name] = Ulimit{
				Name: name,
				ulimitValues: ulimitValues{
					Soft: soft,
					Hard: hard,
				},
			}
		}
		keys := make([]string, 0, len(ulimits))
		for key := range ulimits {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			u.Elements = append(u.Elements, ulimits[key])
		}
	default:
		return fmt.Errorf("Failed to unmarshal Ulimit: %#v", value)
	}
	return nil
}

// Ulimit represents ulimit information.
type Ulimit struct {
	ulimitValues
	Name string
}

type ulimitValues struct {
	Soft int64 `yaml:"soft"`
	Hard int64 `yaml:"hard"`
}

// MarshalYAML implements the Marshaller interface.
func (u Ulimit) MarshalYAML() (tag string, value interface{}, err error) {
	if u.Soft == u.Hard {
		return "", u.Soft, nil
	}
	return "", u.ulimitValues, err
}

// NewUlimit creates a Ulimit based on the specified parts.
func NewUlimit(name string, soft int64, hard int64) Ulimit {
	return Ulimit{
		Name: name,
		ulimitValues: ulimitValues{
			Soft: soft,
			Hard: hard,
		},
	}
}

// Command represents a docker command, can be a string or an array of strings.
// FIXME why not use Stringorslice (type Command struct { Stringorslice }
type Command struct {
	parts []string
}

// MarshalYAML implements the Marshaller interface.
func (s Command) MarshalYAML() (tag string, value interface{}, err error) {
	return "", s.parts, nil
}

// UnmarshalYAML implements the Unmarshaller interface.
func (s *Command) UnmarshalYAML(tag string, value interface{}) error {
	switch value := value.(type) {
	case []interface{}:
		parts, err := toStrings(value)
		if err != nil {
			return err
		}
		s.parts = parts
	case string:
		parts, err := shlex.Split(value)
		if err != nil {
			return err
		}
		s.parts = parts
	default:
		return fmt.Errorf("Failed to unmarshal Command: %#v", value)
	}
	return nil
}

// ToString returns the parts of the command as a string (joined by spaces).
func (s *Command) ToString() string {
	return strings.Join(s.parts, " ")
}

// Slice gets the parts of the Command as a Slice of string.
func (s *Command) Slice() []string {
	return s.parts
}

// NewCommand creates a Command based on the specified parts (as strings).
func NewCommand(parts ...string) Command {
	return Command{parts}
}

// SliceorMap represents a slice or a map of strings.
type SliceorMap struct {
	parts map[string]string
}

// MarshalYAML implements the Marshaller interface.
func (s SliceorMap) MarshalYAML() (tag string, value interface{}, err error) {
	return "", s.parts, nil
}

// UnmarshalYAML implements the Unmarshaller interface.
func (s *SliceorMap) UnmarshalYAML(tag string, value interface{}) error {
	switch value := value.(type) {
	case map[interface{}]interface{}:
		parts := map[string]string{}
		for k, v := range value {
			if sk, ok := k.(string); ok {
				if sv, ok := v.(string); ok {
					parts[sk] = sv
				} else {
					return fmt.Errorf("Cannot unmarshal '%v' of type %T into a string value", v, v)
				}
			} else {
				return fmt.Errorf("Cannot unmarshal '%v' of type %T into a string value", k, k)
			}
		}
		s.parts = parts
	case []interface{}:
		parts := map[string]string{}
		for _, s := range value {
			if str, ok := s.(string); ok {
				str := strings.TrimSpace(str)
				keyValueSlice := strings.SplitN(str, "=", 2)

				key := keyValueSlice[0]
				val := ""
				if len(keyValueSlice) == 2 {
					val = keyValueSlice[1]
				}
				parts[key] = val
			} else {
				return fmt.Errorf("Cannot unmarshal '%v' of type %T into a string value", s, s)
			}
		}
		s.parts = parts
	default:
		return fmt.Errorf("Failed to unmarshal SliceorMap: %#v", value)
	}
	return nil
}

// MapParts gets the parts of the SliceorMap as a Map of string.
func (s *SliceorMap) MapParts() map[string]string {
	if s == nil {
		return nil
	}
	return s.parts
}

// NewSliceorMap creates a new SliceorMap based on the specified parts (as map of string).
func NewSliceorMap(parts map[string]string) SliceorMap {
	return SliceorMap{parts}
}

// MaporEqualSlice represents a slice of strings that gets unmarshal from a
// YAML map into 'key=value' string.
type MaporEqualSlice struct {
	parts []string
}

// MarshalYAML implements the Marshaller interface.
func (s MaporEqualSlice) MarshalYAML() (tag string, value interface{}, err error) {
	return "", s.parts, nil
}

func toSepMapParts(value map[interface{}]interface{}, sep string) ([]string, error) {
	if len(value) == 0 {
		return nil, nil
	}
	parts := make([]string, 0, len(value))
	for k, v := range value {
		if sk, ok := k.(string); ok {
			if sv, ok := v.(string); ok {
				parts = append(parts, sk+sep+sv)
			} else {
				return nil, fmt.Errorf("Cannot unmarshal '%v' of type %T into a string value", v, v)
			}
		} else {
			return nil, fmt.Errorf("Cannot unmarshal '%v' of type %T into a string value", k, k)
		}
	}
	return parts, nil
}

// UnmarshalYAML implements the Unmarshaller interface.
func (s *MaporEqualSlice) UnmarshalYAML(tag string, value interface{}) error {
	switch value := value.(type) {
	case []interface{}:
		parts, err := toStrings(value)
		if err != nil {
			return err
		}
		s.parts = parts
	case map[interface{}]interface{}:
		parts, err := toSepMapParts(value, "=")
		if err != nil {
			return err
		}
		s.parts = parts
	default:
		return fmt.Errorf("Failed to unmarshal MaporEqualSlice: %#v", value)
	}
	return nil
}

// Slice gets the parts of the MaporEqualSlice as a Slice of string.
func (s *MaporEqualSlice) Slice() []string {
	return s.parts
}

// NewMaporEqualSlice creates a new MaporEqualSlice based on the specified parts.
func NewMaporEqualSlice(parts []string) MaporEqualSlice {
	return MaporEqualSlice{parts}
}

// MaporColonSlice represents a slice of strings that gets unmarshal from a
// YAML map into 'key:value' string.
type MaporColonSlice struct {
	parts []string
}

// MarshalYAML implements the Marshaller interface.
func (s MaporColonSlice) MarshalYAML() (tag string, value interface{}, err error) {
	return "", s.parts, nil
}

// UnmarshalYAML implements the Unmarshaller interface.
func (s *MaporColonSlice) UnmarshalYAML(tag string, value interface{}) error {
	switch value := value.(type) {
	case []interface{}:
		parts, err := toStrings(value)
		if err != nil {
			return err
		}
		s.parts = parts
	case map[interface{}]interface{}:
		parts, err := toSepMapParts(value, ":")
		if err != nil {
			return err
		}
		s.parts = parts
	default:
		return fmt.Errorf("Failed to unmarshal MaporColonSlice: %#v", value)
	}
	return nil
}

// Slice gets the parts of the MaporColonSlice as a Slice of string.
func (s *MaporColonSlice) Slice() []string {
	return s.parts
}

// NewMaporColonSlice creates a new MaporColonSlice based on the specified parts.
func NewMaporColonSlice(parts []string) MaporColonSlice {
	return MaporColonSlice{parts}
}

// MaporSpaceSlice represents a slice of strings that gets unmarshal from a
// YAML map into 'key value' string.
type MaporSpaceSlice struct {
	parts []string
}

// MarshalYAML implements the Marshaller interface.
func (s MaporSpaceSlice) MarshalYAML() (tag string, value interface{}, err error) {
	return "", s.parts, nil
}

// UnmarshalYAML implements the Unmarshaller interface.
func (s *MaporSpaceSlice) UnmarshalYAML(tag string, value interface{}) error {
	switch value := value.(type) {
	case []interface{}:
		parts, err := toStrings(value)
		if err != nil {
			return err
		}
		s.parts = parts
	case map[interface{}]interface{}:
		parts, err := toSepMapParts(value, " ")
		if err != nil {
			return err
		}
		s.parts = parts
	default:
		return fmt.Errorf("Failed to unmarshal MaporSpaceSlice: %#v", value)
	}
	return nil
}

// Slice gets the parts of the MaporSpaceSlice as a Slice of string.
func (s *MaporSpaceSlice) Slice() []string {
	return s.parts
}

// NewMaporSpaceSlice creates a new MaporSpaceSlice based on the specified parts.
func NewMaporSpaceSlice(parts []string) MaporSpaceSlice {
	return MaporSpaceSlice{parts}
}
