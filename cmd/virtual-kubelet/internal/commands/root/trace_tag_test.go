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

package root

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestTraceTagFlag(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	c := &Opts{}
	installFlags(flags, c)

	if err := flags.Parse([]string{"--trace-tag", "team=edge", "--trace-tag", "region=eu"}); err != nil {
		t.Fatalf("parsing --trace-tag failed: %v", err)
	}
	if c.TraceConfig.Tags["team"] != "edge" || c.TraceConfig.Tags["region"] != "eu" {
		t.Errorf("unexpected trace tags: %v", c.TraceConfig.Tags)
	}
}
