// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package trace

import (
	"context"

	"github.com/virtual-kubelet/virtual-kubelet/log"
)

type nopTracer struct{}

func (nopTracer) StartSpan(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, &nopSpan{}
}

type nopSpan struct{}

func (nopSpan) End()               {}
func (nopSpan) SetStatus(error)    {}
func (nopSpan) Logger() log.Logger { return nil }

func (nopSpan) WithField(ctx context.Context, _ string, _ interface{}) context.Context { return ctx }
func (nopSpan) WithFields(ctx context.Context, _ log.Fields) context.Context           { return ctx }
