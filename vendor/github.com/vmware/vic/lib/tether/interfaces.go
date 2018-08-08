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

package tether

import (
	"context"
	"io"
	"net/url"
	"os"

	"github.com/vmware/vic/pkg/dio"
)

// UtilityFn is the sigature of a function that can be used to launch a utility process
type UtilityFn func() (*os.Process, error)

// Operations defines the set of operations that Tether depends upon. These are split out for:
// * portability
// * dependency injection (primarily for testing)
// * behavioural control (e.g. what behaviour is required when a session exits)
type Operations interface {
	Setup(Config) error
	Cleanup() error
	// Log returns the tether debug log writer
	Log() (io.Writer, error)

	SetHostname(hostname string, aliases ...string) error
	SetupFirewall(ctx context.Context, config *ExecutorConfig) error
	Apply(endpoint *NetworkEndpoint) error
	MountLabel(ctx context.Context, label, target string) error
	MountTarget(ctx context.Context, source url.URL, target string, mountOptions string) error
	CopyExistingContent(source string) error
	Fork() error
	// Returns two DynamicMultiWriters for stdout and stderr
	SessionLog(session *SessionConfig) (dio.DynamicMultiWriter, dio.DynamicMultiWriter, error)
	// Returns a function to invoke after the session state has been persisted
	HandleSessionExit(config *ExecutorConfig, session *SessionConfig) func()
	ProcessEnv(session *SessionConfig) []string
	// LaunchUtility starts a process and provides a way to block on completion and retrieve
	// it's exit code. This is needed to co-exist with a childreaper.
	LaunchUtility(UtilityFn) (<-chan int, error)
	// HandleUtilityExit will process the utility exit. If the pid cannot be matched to a launched
	// utility process then this returns false and does nothing.
	HandleUtilityExit(pid, exitCode int) bool
}

// System is a very preliminary interface that provides extensions with a means by which to
// perform system style operations.
// This should be designed with intent to provide sufficient abstraction that vic-init, tether
// can be run by vcsim. Odds are that a significant portion of the methods on this interface
// will be defined in vcsim and this interface will simply embed the vcsim interface.
// There may well be overlap with shared.Sys that needs to be resolved
//
// For now its sole purpose is to allow extensions access to the specific portions of
// BaseOperations that they actively use.
type System interface {
	// LaunchUtility starts a process and provides a way to block on completion and retrieve
	// it's exit code. This is needed to co-exist with a childreaper.
	LaunchUtility(UtilityFn) (<-chan int, error)

	MountLabel(ctx context.Context, label, target string) error
}

// Tether presents the consumption interface for code needing to run a tether
type Tether interface {
	Start() error
	Stop() error
	Reload()
	Register(name string, ext Extension)
}

// Extension is a very simple extension interface for supporting code that need to be
// notified when the configuration is reloaded.
type Extension interface {
	Start(System) error
	Reload(config *ExecutorConfig) error
	Stop() error
}

type Config interface {
	UpdateNetworkEndpoint(e *NetworkEndpoint) error
	Flush() error
}
