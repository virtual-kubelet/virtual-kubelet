package resourcegroups

import "github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/api"

// Group holds resource group information.
type Group struct {
	api.ResponseMetadata `json:"-"`
	ID                   string            `json:"id,omitempty"`
	Name                 string            `json:"name,omitempty"`
	Properties           *GroupProperties  `json:"properties,omitempty"`
	Location             string            `json:"location,omitempty"`
	ManagedBy            string            `json:"managedBy,omitempty"`
	Tags                 map[string]string `json:"tags,omitempty"`
}

// GroupProperties deines the properties for an Azure resource group.
type GroupProperties struct {
	ProvisioningState string `json:"provisioningState,omitempty"`
}
