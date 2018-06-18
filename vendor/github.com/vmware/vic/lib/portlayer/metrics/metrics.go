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

package metrics

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/performance"
	"github.com/vmware/vic/pkg/vsphere/session"
)

var (
	Supervisor *super

	initializer struct {
		err  error
		once sync.Once
	}
)

// super manages the lifecycle and access to the
// available metrics collectors
type super struct {
	vms *performance.VMCollector
}

type UnsupportedTypeError struct {
	subscriber interface{}
}

func (ute UnsupportedTypeError) Error() string {
	t := reflect.TypeOf(ute.subscriber)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return fmt.Sprintf("type %s is not supported by metrics", t)
}

func Init(ctx context.Context, session *session.Session) error {
	defer trace.End(trace.Begin(""))
	initializer.once.Do(func() {
		var err error
		defer func() {
			if err != nil {
				initializer.err = err
			}
		}()
		Supervisor = newSupervisor(session)

	})
	return initializer.err

}

func newSupervisor(session *session.Session) *super {
	defer trace.End(trace.Begin(""))
	// create the vm metric collector
	v := performance.NewVMCollector(session)
	return &super{
		vms: v,
	}
}

func (s *super) Subscribe(op trace.Operation, subscriber interface{}) (chan interface{}, error) {
	switch sub := subscriber.(type) {
	case *exec.Container:
		return s.vms.Subscribe(op, sub.VMReference(), sub.String())
	}

	err := UnsupportedTypeError{
		subscriber: subscriber,
	}
	op.Errorf("%s", err)

	return nil, err
}

func (s *super) Unsubscribe(op trace.Operation, subscriber interface{}, ch chan interface{}) {
	switch sub := subscriber.(type) {
	case *exec.Container:
		s.vms.Unsubscribe(op, sub.VMReference(), ch)
	default:
		err := UnsupportedTypeError{
			subscriber: subscriber,
		}
		op.Errorf("%s", err)
	}
	return
}
