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

package registry

type Set []Entry

type Merger interface {
	Merge(Entry, Entry) (Entry, error)
}

type defaultMerger struct{}

func (m *defaultMerger) Merge(orig, other Entry) (Entry, error) {
	if orig.String() == other.String() {
		return other, nil
	}

	return nil, nil
}

func (s Set) Match(m string) bool {
	for _, e := range s {
		if e.Match(m) {
			return true
		}
	}

	return false
}

func (s Set) Merge(other Set, merger Merger) (Set, error) {
	if merger == nil {
		merger = &defaultMerger{}
	}

	res := make([]Entry, len(s))
	var adds Set
	copy(res, s)
	for _, o := range other {
		merged := false
		for i := 0; i < len(res); i++ {
			e, err := merger.Merge(res[i], o)
			if err != nil {
				return nil, err
			}

			if e != nil {
				res[i] = e
				merged = true
				break
			}
		}

		if !merged {
			adds = append(adds, o)
		}
	}

	return append(res, adds...), nil
}

func (s Set) Strings() []string {
	res := make([]string, len(s))
	for i := range s {
		res[i] = s[i].String()
	}

	return res
}
