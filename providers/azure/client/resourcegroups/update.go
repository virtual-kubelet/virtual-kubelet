package resourcegroups

// UpdateResourceGroup updates an Azure resource group with the
// provided properties.
// From: https://docs.microsoft.com/en-us/rest/api/resources/resourcegroups/createorupdate
func (c *Client) UpdateResourceGroup(resourceGroup string, properties Group) (*Group, error) {
	return c.CreateResourceGroup(resourceGroup, properties)
}
