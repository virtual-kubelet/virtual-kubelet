variable "cluster_client_certificate" {
  type        = "string"
  description = "Cluster client Certificate"
  default     = "eastus"
}

variable "cluster_client_key" {
  type        = "string"
  description = "Cluster client Certificate Key"
}

variable "cluster_ca" {
  type        = "string"
  description = "Cluster Certificate Authority"
}

variable "cluster_host" {
  type        = "string"
  description = "Cluster Admin API host"
}

variable "virtualkubelet_docker_image" {
  type        = "string"
  description = "The docker image to use for deploying the virtual kubelet"
}

variable "azure_batch_account_name" {
  type        = "string"
  description = "The name of the Azure Batch account to use"
}

variable "azure_batch_pool_id" {
  type        = "string"
  description = "The PoolID to use in Azure batch"
  default     = "pool1"
}

variable "resource_group_location" {
  description = "Resource group location"
  type        = "string"
}
