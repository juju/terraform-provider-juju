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
  description = "Cloud to bootstrap juju controller on"
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

/* Example of LXD cloud
  cloud = {
    auth_types = ["certificate"]
    name       = "lxd-cloud"
    type       = "lxd"
    endpoint   = "https://10.0.0.1:8383"
    region = {
      name     = "default"
      endpoint = "https://10.0.0.1:8383"
    }
  }
*/

variable "cloud_credential" {
  description = "Cloud credentials to bootstrap juju controller"
  type = object({
    attributes = map(string)
    auth_type  = string
    name       = string
  })
}

/* Example of LXD cloud credentials
  cloud_credential = {
    auth_type = "interactive"
    name      = "lxd-token"
    attributes = {
      trust-token = trimspace(file("/path/to/token"))
    }
  }
*/

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

/* Example of controller_model_config
  controller_model_config = {
    default-base = "ubuntu@22.04"
    lxd-snap-channel = "5.0/stable"
    cloudinit-userdata = <<EOT
#cloud-config

  ca-certs:
    trusted:
    - |
      -----BEGIN CERTIFICATE-----
      ROOT CA
      -----END CERTIFICATE-----
      -----BEGIN CERTIFICATE-----
      INTERMEDIATE CA
      -----END CERTIFICATE-----
EOT
  }
*/

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

/* Example of model_default
  model_default = {
    default-base = "ubuntu@22.04"
    lxd-snap-channel = "5.0/stable"
    cloudinit-userdata = <<EOT
#cloud-config

  ca-certs:
    trusted:
    - |
      -----BEGIN CERTIFICATE-----
      ROOT CA
      -----END CERTIFICATE-----
      -----BEGIN CERTIFICATE-----
      INTERMEDIATE CA
      -----END CERTIFICATE-----
EOT
  }
*/

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
}
