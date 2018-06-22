variable "client_id" {
  type        = "string"
  description = "Client ID"
}

variable "client_secret" {
  type        = "string"
  description = "Client secret."
}

variable "resource_group_name" {
  description = "Resource group name"
  type        = "string"
}

variable "resource_group_location" {
  description = "Resource group location"
  type        = "string"
}

variable "virtualkubelet_docker_image" {
  type = "string"
}
