variable "path_juju_binary" {
  description = "Path to Juju binary"
  type        = string
  default     = "/snap/juju/current/bin/juju"
}

variable "name" {
  description = "Name of the juju controller to bootstrap"
  type        = string
}

variable "cloud" {
  description = <<EOT
Cloud to bootstrap juju controller on.

Fields:
- auth_types: (set of string) The authentication type(s) supported by the cloud.
- name: (string) The name of the cloud.
- type: (string) The type of the cloud.
- ca_certificates: (optional set of string) CA certificates for the cloud.
- config: (optional map of string) Configuration options for the cloud.
- endpoint: (optional string) The API endpoint for the cloud.
- host_cloud_region: (optional string) The host cloud region for the cloud.
- region: (optional object)
    - name: (string) The name of the region.
    - endpoint: (optional string) The API endpoint for the region.
    - identity_endpoint: (optional string) The identity endpoint for the region.
    - storage_endpoint: (optional string) The storage endpoint for the region.
EOT
  type = object({
    auth_types        = set(string)
    name              = string
    type              = string
    ca_certificates   = optional(set(string))
    config            = optional(map(string))
    endpoint          = optional(string)
    host_cloud_region = optional(string)
    region = optional(object({
      name              = string
      endpoint          = optional(string)
      identity_endpoint = optional(string)
      storage_endpoint  = optional(string)
    }))
  })
}

variable "cloud_credential" {
  description = "Cloud credentials to bootstrap juju controller"
  type = object({
    attributes = map(string)
    auth_type  = string
    name       = string
  })
}

variable "agent_version" {
  description = "juju agent version to use"
  type        = string
  default     = null
}

variable "bootstrap_base" {
  description = "Bootstrap base to use for juju controller"
  type        = string
  default     = null
}

variable "bootstrap_config" {
  description = "Bootstrap config to use for juju controller"
  type        = map(string)
  default     = null
}

variable "bootstrap_constraints" {
  description = "Bootstrap constraints to use for juju controller"
  type        = map(string)
  default     = null
}

variable "controller_config" {
  description = "Controller config"
  type        = map(string)
  default     = null
}

variable "controller_model_config" {
  description = "Controller model config"
  type        = map(string)
  default     = null
}

variable "destroy_flags" {
  description = "Flags to pass to juju destroy-controller command"
  type = object({
    destroy_all_models = optional(bool)
    destroy_storage    = optional(bool)
    force              = optional(bool)
    model_timeout      = optional(number)
    release_storage    = optional(bool)
  })
  default = {
    destroy_all_models = true
    destroy_storage    = true
  }
}

variable "model_constraints" {
  description = "Model constraints to set for all models"
  type        = map(string)
  default     = null
}

variable "model_default" {
  description = "Model defaults to set for all models"
  type        = map(string)
  default     = null
}

variable "storage_pool" {
  description = "Storage pool to use for juju controller"
  type = object({
    name       = string
    type       = string
    attributes = optional(map(string))
  })
  default = null
}

variable "controller_num_units" {
  description = "Number of controller units to deploy"
  type        = number

  validation {
    condition     = var.controller_num_units > 0 && floor(var.controller_num_units) == var.controller_num_units
    error_message = "controller_num_units must be a positive integer greater than 0."
  }
}