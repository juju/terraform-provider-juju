# Example: export every supported resource from a single Juju model.
#
# Usage:
#
#   TF_VAR_model_uuid="<model-uuid>" terraform query \
#     --generate-config-out=exported.tf
#
# Then rewrite literal UUIDs/names into cross-resource references:
#
#   go run ./juju-tf-refwriter exported.tf
#
# See .github/instructions/tf-model-export-fixup.instructions.md for the full
# workflow to get `terraform plan` clean (provider block, dropping the alpha
# system space, iterating on plan errors).

variable "model_uuid" {
  description = "UUID of the model to export"
  type        = string
}

list "juju_model" "model" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

list "juju_application" "all_apps" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

list "juju_machine" "all_machines" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

list "juju_integration" "all_integrations" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

list "juju_secret" "all_secrets" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

list "juju_offer" "all_offers" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

list "juju_ssh_key" "all_ssh_keys" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

list "juju_space" "all_spaces" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

list "juju_storage_pool" "all_storage_pools" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}
