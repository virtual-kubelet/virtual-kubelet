package types

import (
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/hyperhq/hyper-api/types/container"
	"github.com/hyperhq/hyper-api/types/filters"
	"github.com/hyperhq/hyper-api/types/network"
	"github.com/hyperhq/hyper-api/types/strslice"
)

type FuncConfig struct {
	Tty          bool                  `json:"Tty,omitempty"`
	Env          *[]string             `json:"Env,omitempty"`
	ExposedPorts map[nat.Port]struct{} `json:"ExposedPorts,omitempty"`
	Cmd          strslice.StrSlice     `json:"Cmd,omitempty"`
	Image        string                `json:"Image,omitempty"`
	Entrypoint   strslice.StrSlice     `json:"Entrypoint,omitempty"`
	WorkingDir   string                `json:"WorkingDir,omitempty"`
	Labels       map[string]string     `json:"Labels,omitempty"`
	StopSignal   string                `json:"StopSignal,omitempty"`
}

type FuncHostConfig struct {
	VolumesFrom     []string              `json:"VolumesFrom,omitempty"`
	PortBindings    nat.PortMap           `json:"PortBindings,omitempty"`
	Links           []string              `json:"Links,omitempty"`
	PublishAllPorts bool                  `json:"PublishAllPorts,omitempty"`
	NetworkMode     container.NetworkMode `json:"NetworkMode,omitempty"`
}

type Func struct {
	// Func name, required, unique, immutable, max length: 255, format: [a-z0-9]([-a-z0-9]*[a-z0-9])?
	Name string `json:"Name"`

	// Container size, optional, default: s4
	ContainerSize string `json:"ContainerSize,omitempty"`

	// The maximum execution duration of function call
	Timeout int `json:"Timeout,omitempty"`

	// The UUID of func
	UUID string `json:"UUID,omitempty"`

	// The created time
	Created time.Time `json:"Created,omitempty"`

	// Weather the UUID should be regenerated
	Refresh bool `json:"Refresh,omitempty"`

	// The container config
	Config FuncConfig `json:"Config,omitempty"`

	HostConfig FuncHostConfig `json:"HostConfig,omitempty"`

	NetworkingConfig network.NetworkingConfig `json:"NetworkingConfig,omitempty"`
}

type FuncListOptions struct {
	Filters filters.Args
}

type FuncCallResponse struct {
	CallId string `json:"CallId"`
}

type FuncLogsResponse struct {
	Time        time.Time `json:"Time"`
	Event       string    `json:"Event"`
	CallId      string    `json:"CallId"`
	ShortStdin  string    `json:"ShortStdin"`
	ShortStdout string    `json:"ShortStdout"`
	ShortStderr string    `json:"ShortStderr"`
	Message     string    `json:"Message"`
}

type FuncStatusResponse struct {
	Total    int `json:"Total"`
	Pending  int `json:"Pending"`
	Running  int `json:"Running"`
	Finished int `json:"Finished"`
	Failed   int `json:"Failed"`
}
