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

package main

import "github.com/vmware/vic/lib/tether"

// pathPrefix is present to allow the various files referenced by tether to be placed
// in specific directories, primarily for testing.
var pathPrefix string

func (t *operations) Cleanup() error {
	return t.BaseOperations.Cleanup()
}

func (t *operations) Apply(endpoint *tether.NetworkEndpoint) error {
	return t.BaseOperations.Apply(endpoint)
}

// HandleSessionExit controls the behaviour on session exit - for the tether if the session exiting
// is the primary session (i.e. SessionID matches ExecutorID) then we exit everything.
func (t *operations) HandleSessionExit(config *tether.ExecutorConfig, session *tether.SessionConfig) func() {
	// if the session that's exiting is the primary session, stop the tether
	return func() {
		pod := false
		for _, s := range config.Sessions {
			if s.ExecutionEnvironment != "" {
				pod = true
			}
			if s.StopTime <= s.StartTime {
				return
			}
		}
		for _, s := range config.Execs {
			if pod && s.StopTime <= s.StartTime {
				return
			}
		}
		tthr.Stop()
	}
}
