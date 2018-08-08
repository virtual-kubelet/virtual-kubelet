package project

import "github.com/docker/engine-api/client"

// ClientFactory is a factory to create docker clients.
type ClientFactory interface {
	// Create constructs a Docker client for the given service. The passed in
	// config may be nil in which case a generic client for the project should
	// be returned.
	Create(service Service) client.APIClient
}
