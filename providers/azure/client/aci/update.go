package aci

// UpdateContainerGroup updates an Azure Container Instance with the
// provided properties.
// From: https://docs.microsoft.com/en-us/rest/api/container-instances/containergroups/createorupdate
func (c *Client) UpdateContainerGroup(resourceGroup, containerGroupName string, containerGroup ContainerGroup) (*ContainerGroup, error) {
	return c.CreateContainerGroup(resourceGroup, containerGroupName, containerGroup)
}
