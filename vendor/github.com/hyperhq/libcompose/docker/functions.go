package docker

import (
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	"golang.org/x/net/context"
)

// GetContainersByFilter looks up the hosts containers with the specified filters and
// returns a list of container matching it, or an error.
func GetContainersByFilter(clientInstance client.APIClient, containerFilters ...map[string][]string) ([]types.Container, error) {
	filterArgs := filters.NewArgs()

	// FIXME(vdemeester) I don't like 3 for loops >_<
	for _, filter := range containerFilters {
		for key, filterValue := range filter {
			for _, value := range filterValue {
				filterArgs.Add(key, value)
			}
		}
	}

	return clientInstance.ContainerList(context.Background(), types.ContainerListOptions{
		All:    true,
		Filter: filterArgs,
	})
}

// GetContainer looks up the hosts containers with the specified ID
// or name and returns it, or an error.
func GetContainer(clientInstance client.APIClient, id string) (*types.ContainerJSON, error) {
	container, err := clientInstance.ContainerInspect(context.Background(), id)
	if err != nil {
		if client.IsErrContainerNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &container, nil
}
