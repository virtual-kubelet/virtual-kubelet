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

func TestMetricsAddrFlagIsDeprecated(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	installFlags(flags, &Opts{})

	f := flags.Lookup("metrics-addr")
	if f == nil {
		t.Fatal("metrics-addr flag is not registered")
	}
	if f.Deprecated == "" || !f.Hidden {
		t.Errorf("expected metrics-addr to be deprecated and hidden, got deprecated=%q hidden=%v", f.Deprecated, f.Hidden)
	}

	// Existing command lines that pass the flag must keep working.
	if err := flags.Parse([]string{"--metrics-addr", ":10255"}); err != nil {
		t.Errorf("parsing --metrics-addr failed: %v", err)
	}
}
