package docker

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
)

// GetContainersByFilter looks up the hosts containers with the specified filters and
// returns a list of container matching it, or an error.
func GetContainersByFilter(client client.APIClient, containerFilters ...map[string][]string) ([]types.Container, error) {
	filterArgs := filters.NewArgs()

	// FIXME(vdemeester) I don't like 3 for loops >_<
	for _, filter := range containerFilters {
		for key, filterValue := range filter {
			for _, value := range filterValue {
				filterArgs.Add(key, value)
			}
		}
	}

	return client.ContainerList(context.Background(), types.ContainerListOptions{
		All:    true,
		Filter: filterArgs,
	})
}

// GetContainerByName looks up the hosts containers with the specified name and
// returns it, or an error.
func GetContainerByName(client client.APIClient, name string) (*types.Container, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("name", fmt.Sprintf("%s", name))

	containers, err := client.ContainerList(context.Background(), types.ContainerListOptions{
		All:    true,
		Filter: filterArgs,
	})
	if err != nil {
		return nil, err
	}

	if len(containers) == 0 {
		return nil, nil
	}

	return &containers[0], nil
}

// GetContainerByID looks up the hosts containers with the specified Id and
// returns it, or an error.
func GetContainerByID(client client.APIClient, id string) (*types.Container, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("id", id)

	containers, err := client.ContainerList(context.Background(), types.ContainerListOptions{
		All:    true,
		Filter: filterArgs,
	})
	if err != nil {
		return nil, err
	}

	if len(containers) == 0 {
		return nil, nil
	}

	return &containers[0], nil
}
