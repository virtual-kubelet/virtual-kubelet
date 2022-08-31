// Copyright © 2021 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package klogv2

import (
	"testing"

	"github.com/virtual-kubelet/virtual-kubelet/log"
)

func TestFieldMap_String(t *testing.T) {
	var tests = []struct {
		desc     string
		fields   *fieldMap
		expected string
	}{
		{
			desc:     "fieldMap with nil fields",
			fields:   &fieldMap{Fields: nil},
			expected: "",
		},
		{
			desc:     "fieldMap with empty fields",
			fields:   &fieldMap{Fields: make(log.Fields)},
			expected: "",
		},
		{
			desc:     "fieldMap with single field",
			fields:   &fieldMap{Fields: map[string]interface{}{"one": 1}},
			expected: " [one=1]",
		},
		{
			desc:     "fieldMap with two fields",
			fields:   &fieldMap{Fields: map[string]interface{}{"one": 1, "two": 2}},
			expected: " [one=1 two=2]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Assert fields haven't been processed yet.
			if len(tt.fields.processedFields) > 0 {
				t.Fatal("fields shouldn't have been processed yet")
			}
			// Assert fields have been processed, if any.
			actual := tt.fields.String()
			if len(tt.fields.Fields) > 0 && len(tt.fields.processedFields) == 0 {
				t.Fatal("fields should have been processed by now")
			}
			// Assert processFields yields desired results.
			if actual != tt.expected {
				t.Fatalf("expected: %s, got: %s", actual, tt.expected)
			}
		})
	}
}
