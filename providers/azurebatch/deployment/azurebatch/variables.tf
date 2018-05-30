variable "pool_id" {
  type        = "string"
  description = "Name of the Azure Batch pool to create."
  default     = "pool1"
}

variable "vm_sku" {
  type        = "string"
  description = "VM SKU to use - Default to NC6 GPU SKU."
  default     = "STANDARD_NC6"
}

variable "pool_bootstrap_script_url" {
  type        = "string"
  description = "Publicly accessible url used for boostrapping nodes in the pool. Installing GPU drivers, for example."
}

variable "storage_account_id" {
  type        = "string"
  description = "Name of the storage account to be used by Azure Batch"
}

variable "resource_group_name" {
  type        = "string"
  description = "Name of the azure resource group."
  default     = "akc-rg"
}

variable "resource_group_location" {
  type        = "string"
  description = "Location of the azure resource group."
  default     = "eastus"
}

variable "low_priority_node_count" {
  type        = "string"
  description = "The number of low priority nodes to allocate to the pool"
}

variable "dedicated_node_count" {
  type        = "string"
  description = "The number dedicated nodes to allocate to the pool"
}
