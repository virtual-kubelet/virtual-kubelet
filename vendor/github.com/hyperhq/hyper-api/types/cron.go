package types

import (
	"time"

	"github.com/hyperhq/hyper-api/types/container"
	"github.com/hyperhq/hyper-api/types/filters"
	"github.com/hyperhq/hyper-api/types/network"
)

type Cron struct {
	// Job name. Must be unique, acts as the id.
	Name string `json:"Name"`

	// Cron expression for the job. When to run the job.
	Schedule string `json:"Schedule"`

	// AccessKey
	AccessKey string `json:"AccessKey"`

	// SecretKey
	SecretKey string `json:"SecretKey"`

	ContainerName string                    `json:"ContainerName"`
	Config        *container.Config         `json:"Config"`
	HostConfig    *container.HostConfig     `json:"HostConfig"`
	NetConfig     *network.NetworkingConfig `json:"NetConfig"`

	// Owner of the job.
	Owner string `json:"Owner"`

	// Owner email of the job.
	OwnerEmail string `json:"OwnerEmail"`

	// MailPolicy
	MailPolicy string `json:"MailPolicy"`

	// Number of successful executions of this job.
	SuccessCount int `json:"SuccessCount"`

	// Number of errors running this job.
	ErrorCount int `json:"ErrorCount"`

	// Last time this job executed.
	LastRun time.Time `json:"LastRun"`

	Created time.Time `json:"Created"`

	// Is this job disabled?
	Disabled bool `json:"Disabled"`

	// Tags of the target servers to run this job against.
	Tags map[string]string `json:"Tags"`
}

type CronListOptions struct {
	Filters filters.Args
}

type Event struct {
	StartedAt  int64  `json:"StartedAt"`
	FinishedAt int64  `json:"FinishedAt"`
	Status     string `json:"Status"`
	Job        string `json:"Job"`
	Container  string `json:"Container"`
	Message    string `json:"Message"`
}
