package aci

import "context"

// UpdateContainerGroup updates an Azure Container Instance with the
// provided properties.
// From: https://docs.microsoft.com/en-us/rest/api/container-instances/containergroups/createorupdate
func (c *Client) UpdateContainerGroup(ctx context.Context, resourceGroup, containerGroupName string, containerGroup ContainerGroup) (*ContainerGroup, error) {
	return c.CreateContainerGroup(ctx, resourceGroup, containerGroupName, containerGroup)
}
