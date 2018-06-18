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

package communication

import (
	"fmt"
	"reflect"
	"testing"
)

func TestLazySessionInteractor_Initialize(t *testing.T) {
	type fields struct {
		si SessionInteractor
		fn LazyInitializer
	}
	tests := []struct {
		name    string
		fields  fields
		want    SessionInteractor
		wantErr bool
	}{
		{"FnIsNil", fields{si: &interaction{}}, &interaction{}, false},
		{"SiIsNil", fields{si: nil, fn: func() (SessionInteractor, error) { return &interaction{}, nil }}, &interaction{}, false},
		{"FnAndSIAreNotNil", fields{si: &interaction{}, fn: func() (SessionInteractor, error) { return nil, fmt.Errorf("failure") }}, &interaction{}, false},
		{"SiIsNilFnWillFail", fields{si: nil, fn: func() (SessionInteractor, error) { return nil, fmt.Errorf("failure") }}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &LazySessionInteractor{
				si: tt.fields.si,
				fn: tt.fields.fn,
			}
			got, err := l.Initialize()
			if (err != nil) != tt.wantErr {
				t.Errorf("LazySessionInteractor.Initialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LazySessionInteractor.Initialize() = %v, want %v", got, tt.want)
			}
		})
	}
}
