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

import "github.com/vmware/vic/lib/config/executor"

type ExecutorConfig struct {
	// Diagnostics holds basic diagnostics data
	Diagnostics Diagnostics `vic:"0.1" scope:"read-only" key:"diagnostics"`
}

type Diagnostics struct {
	// Should debugging be enabled on whatever component this is and at what level
	DebugLevel int `vic:"0.1" scope:"read-only" key:"debug"`
	// SyslogConfig holds configuration for connecting to a syslog
	// server
	SysLogConfig *executor.SysLogConfig `vic:"0.1" scope:"read-only" key:"syslog"`
}
