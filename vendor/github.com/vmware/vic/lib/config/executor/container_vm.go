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

package executor

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/vmware/vic/pkg/version"
)

type State int

const (
	STARTED State = iota
	EXITED
	KILLED
)

// CopyMode type to define whether to copy data from the base image on mount
type CopyMode int

const (
	// CopyNever Dont copy data on mount
	CopyNever CopyMode = iota + 1

	// CopyNew Copy data to the volume when it is first mounted
	CopyNew
)

// Container network firewall trust configuration value
type TrustLevel int

const (
	Unspecified TrustLevel = iota
	Published
	Open
	Closed
	Outbound
	Peers
)

// Common data between managed entities, across execution environments
type Common struct {
	// A reference to the components hosting execution environment, if any
	ExecutionEnvironment string

	// Unambiguous ID with meaning in the context of its hosting execution environment. Changing this definition will cause container backward compatibility issue. Please don't change this.
	ID string `vic:"0.1" scope:"read-only" key:"id"`

	// Convenience field to record a human readable name
	Name string `vic:"0.1" scope:"read-only" key:"name"`

	// Freeform notes related to the entity
	Notes string `vic:"0.1" scope:"hidden" key:"notes"`
}

// Common data (specifically for a containerVM) between managed entities, across execution environments.
type ExecutorConfigCommon struct {
	// A reference to the components hosting execution environment, if any
	ExecutionEnvironment string

	// Unambiguous ID with meaning in the context of its hosting execution environment
	ID string `vic:"0.1" scope:"read-only" key:"id"`

	// Convenience field to record a human readable name
	Name string `vic:"0.1" scope:"hidden" key:"name"`

	// Freeform notes related to the entity
	Notes string `vic:"0.1" scope:"hidden" key:"notes"`
}

// Diagnostics records some basic control and lifecycle information for diagnostic purposes
type Diagnostics struct {
	// Should debugging be enabled on whatever component this is and at what level
	DebugLevel int `vic:"0.1" scope:"read-only" key:"debug"`

	// RessurectionCount is a log of how many times the entity has been restarted due
	// to error exit
	ResurrectionCount int `vic:"0.1" scope:"read-write" key:"resurrections"`
	// ExitLogs is a best effort record of the time of process death and the cause for
	// restartable entities
	ExitLogs []ExitLog `vic:"0.1" scope:"read-write" key:"exitlogs"`

	// SyslogConfig holds configuration for connecting to a syslog
	// server
	SysLogConfig *SysLogConfig `vic:"0.1" scope:"read-only" key:"syslog"`
}

// SyslogConfig holds the configuration necessary to connect to a syslog server
type SysLogConfig struct {
	// Network can be udp, tcp, udp6, or tcp6
	Network string
	// RAddr is the remote address of the syslog endpoint
	RAddr string
}

// ExitLog records some basic diagnostics about anomalous exit for restartable entities
type ExitLog struct {
	Time       time.Time
	ExitStatus int
	Message    string
}

// MountSpec details a mount that must be executed within the executor
// A mount is a URI -> path mapping with a credential of some kind
// In the case of a labeled disk:
// 	label://<label name> => </mnt/path>
type MountSpec struct {
	// A URI->path mapping, e.g.
	// May contain credentials
	Source url.URL `vic:"0.1" scope:"read-only" key:"source"`

	// The path in the executor at which this should be mounted
	Path string `vic:"0.1" scope:"read-only" key:"dest"`

	// Freeform mode string, which could translate directly to mount options
	// We may want to turn this into a more structured form eventually
	Mode string `vic:"0.1" scope:"read-only" key:"mode"`

	// CopyMode specifies if data should be copied from the base image on first mount
	CopyMode CopyMode `vic:"0.1" scope:"read-only" key:"copymode"`
}

// ContainerVM holds that data tightly associated with a containerVM, but that should not
// be visible to the guest. This is the external complement to ExecutorConfig.
type ContainerVM struct {
	Common

	// The version of the bootstrap image that this container was booted from.
	Version string

	// Name aliases for this specific container, Maps alias to unambiguous name
	// This uses unambiguous name rather than reified network endpoint to persist
	// the intent rather than a point-in-time manifesting of that intent.
	Aliases map[string]string

	// The location of the interaction service that the tether should connect to. Examples:
	// * tcp://x.x.x.x:2377
	// * vmci://moid - should this be an moid or a VMCI CID? Does one insulate us from reboots?
	Interaction url.URL

	// Key is the host key used during communicate back with the Interaction endpoint if any
	// Used if the vSocket agent is responsible for authenticating the connection
	AgentKey []byte
}

// ExecutorConfig holds the data tightly associated with an Executor. This is distinct from Sessions
// in that there is no process inherently associated - this is closer to a ThreadPool than a Thread and
// is the owner of the shared filesystem environment. This is the guest visible complement to ContainerVM.
type ExecutorConfig struct {
	ExecutorConfigCommon `vic:"0.1" scope:"read-only" key:"common"`

	// CreateTime stamp
	CreateTime int64 `vic:"0.1" scope:"read-write" key:"createtime"`

	// Diagnostics holds basic diagnostics data
	Diagnostics Diagnostics `vic:"0.1" scope:"read-only" key:"diagnostics"`

	// Sessions is the set of sessions currently hosted by this executor
	// These are keyed by session ID
	Sessions map[string]*SessionConfig `vic:"0.1" scope:"read-only" key:"sessions"`

	// Execs is the set of non-persistent sessions hosted by this executor
	Execs map[string]*SessionConfig `vic:"0.1" scope:"read-only,non-persistent" key:"execs"`

	// Maps the mount name to the detail mount specification
	Mounts map[string]MountSpec `vic:"0.1" scope:"read-only" key:"mounts"`

	// This describes an executors presence on a network, and contains sufficient
	// information to configure the interface in the guest.
	Networks map[string]*NetworkEndpoint `vic:"0.1" scope:"read-only" key:"networks"`

	// Key is the host key used during communicate back with the Interaction endpoint if any
	// Used if the in-guest tether is responsible for authenticating the connection
	Key []byte `vic:"0.1" scope:"read-only" key:"key"`

	// Layer id that is backing this container VM
	LayerID string `vic:"0.1" scope:"read-only" key:"layerid"`

	// Image id that is backing this container VM
	ImageID string `vic:"0.1" scope:"read-only" key:"imageid"`

	// Blob metadata for the caller
	Annotations map[string]string `vic:"0.1" scope:"hidden" key:"annotations"`

	// Repository requested by user
	// TODO: a bit docker specific
	RepoName string `vic:"0.1" scope:"read-only" key:"repo"`

	// version
	Version *version.Build `vic:"0.1" scope:"read-only" key:"version"`

	// AsymmetricRouting is set to true if the VCH needs to be setup for asymmetric routing
	AsymmetricRouting bool `vic:"0.1" scope:"read-only" key:"asymrouting"`

	// Hostname and domainname provided by personality
	Hostname   string `vic:"0.1" scope:"read-only" key:"hostname"`
	Domainname string `vic:"0.1" scope:"read-only" key:"domainname"`
}

// Cmd is here because the encoding packages seem to have issues with the full exec.Cmd struct
type Cmd struct {
	// Path is the command to run
	Path string `vic:"0.1" scope:"read-only" key:"Path"`

	// Args is the command line arguments including the command in Args[0]
	Args []string `vic:"0.1" scope:"read-only" key:"Args"`

	// Env specifies the environment of the process
	Env []string `vic:"0.1" scope:"read-only" key:"Env"`

	// Dir specifies the working directory of the command
	Dir string `vic:"0.1" scope:"read-only" key:"Dir"`
}

// SessionConfig defines the content of a session - this maps to the root of a process tree
// inside an executor
// This is close to but not perfectly aligned with the new docker/docker/daemon/execdriver/driver:CommonProcessConfig
type SessionConfig struct {
	// The primary session may have the same ID as the executor owning it
	Common `vic:"0.1" scope:"read-only" key:"common"`
	Detail `vic:"0.1" scope:"read-write" key:"detail"`

	// The primary process for the session
	Cmd Cmd `vic:"0.1" scope:"read-only" key:"cmd"`

	// Allow attach
	Attach bool `vic:"0.1" scope:"read-only" key:"attach"`

	OpenStdin bool `vic:"0.1" scope:"read-only" key:"openstdin"`

	// Delay launching the Cmd until an attach request comes
	RunBlock bool `vic:"0.1" scope:"read-write" key:"runblock"`

	// Should this config be activated or not
	Active bool `vic:"0.1" scope:"read-only" key:"active"`

	// Allocate a tty or not
	Tty bool `vic:"0.1" scope:"read-only" key:"tty"`

	ExitStatus int `vic:"0.1" scope:"read-write" key:"status"`

	Started string `vic:"0.1" scope:"read-write" key:"started"`

	Restart bool `vic:"0.1" scope:"read-only" key:"restart"`

	// StopSignal is the signal name or number used to stop container session
	StopSignal string `vic:"0.1" scope:"read-only" key:"stopSignal"`

	// Diagnostics holds basic diagnostics data
	Diagnostics Diagnostics `vic:"0.1" scope:"read-only" key:"diagnostics"`

	// Maps the intent to the signal for this specific app
	// Signals map[int]int

	// Use struct composition to add in the guest specific portions
	// http://attilaolah.eu/2014/09/10/json-and-struct-composition-in-go/
	// ulimits
	// user
	// rootfs - within the container context

	// User and group for setuid programs.
	// Need to go here since UID/GID resolution must be done on appliance
	User  string `vic:"0.1" scope:"read-only" key:"User"`
	Group string `vic:"0.1" scope:"read-only" key:"Group"`
}

type Detail struct {

	// creation, started & stopped timestamps
	CreateTime int64 `vic:"0.1" scope:"read-write" key:"createtime"`
	StartTime  int64 `vic:"0.1" scope:"read-write" key:"starttime"`
	StopTime   int64 `vic:"0.1" scope:"read-write" key:"stoptime"`
}

func (t TrustLevel) String() string {
	switch t {
	case Unspecified:
		return ""
	case Open:
		return "open"
	case Closed:
		return "closed"
	case Published:
		return "published"
	case Outbound:
		return "outbound"
	case Peers:
		return "peers"
	}
	// Default setting, if unknown is published.
	return "published"
}

func ParseTrustLevel(value string) (TrustLevel, error) {
	var trust TrustLevel
	switch strings.ToLower(value) {
	case "open":
		trust = Open
	case "closed":
		trust = Closed
	case "published":
		trust = Published
	case "outbound":
		trust = Outbound
	case "peers":
		trust = Peers
	case "":
		trust = Unspecified
	default:
		return Unspecified, fmt.Errorf("Unrecognized container trust level %s.", value)
	}
	return trust, nil
}
