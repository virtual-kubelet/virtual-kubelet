variable "resource_group_name" {
  description = "Resource group name"
  type        = "string"
}

variable "resource_group_location" {
  description = "Resource group location"
  type        = "string"
}

variable "pool_bootstrap_script_path" {
  description = "The filepath of the pool boostrapping script"
  type        = "string"
}
