package types

import (
	"github.com/hyperhq/hyper-api/types/filters"
	"github.com/hyperhq/hyper-api/types/strslice"
)

const (
	LBProtocolHTTP      string = "http"
	LBProtocolHTTPS     string = "https"
	LBProtocolTCP       string = "tcp"
	LBProtocolHTTPSTERM string = "httpsTerm"

	LBAlgorithmRoundRobin string = "roundrobin"
	LBAlgorithmLeastConn  string = "leastconn"
	LBAlgorithmSource     string = "source"
)

// Service represents the configuration of a service for the remote API
type Service struct {
	Name                string
	Image               string
	WorkingDir          string
	ContainerSize       string
	SSLCert             string
	NetMode             string
	StopSignal          string
	ServicePort         int
	ContainerPort       int
	Replicas            int
	HealthCheckInterval int
	HealthCheckFall     int
	HealthCheckRise     int
	Algorithm           string
	Protocol            string
	Stdin               bool
	Tty                 bool
	SessionAffinity     bool
	Entrypoint          strslice.StrSlice // Entrypoint to run when starting the container
	Cmd                 strslice.StrSlice // Command to run when starting the container
	Env                 []string
	Volumes             map[string]struct{} // List of volumes (mounts) used for the container
	Labels              map[string]string
	SecurityGroups      map[string]struct{}

	IP         string
	FIP        string
	Message    string
	Status     string
	Containers []string
}

type ServiceListOptions struct {
	Filters filters.Args
}

type ServiceUpdate struct {
	Replicas *int
	Image    *string
	FIP      *string
}
