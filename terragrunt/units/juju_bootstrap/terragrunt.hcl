include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "git::https://github.com/canonical/terraform-provider-juju.git//modules/juju_bootstrap?ref=${values.version}"
}

dependencies {
  paths = try(values.dependencies, [])
}

locals {
  optional_inputs = {
    path_juju_binary        = try(values.path_juju_binary, null)
    agent_version           = try(values.agent_version, null)
    bootstrap_base          = try(values.bootstrap_base, null)
    bootstrap_config        = try(values.bootstrap_config, null)
    bootstrap_constraints   = try(values.bootstrap_constraints, null)
    controller_config       = try(values.controller_config, null)
    controller_model_config = try(values.controller_model_config, null)
    destroy_flags           = try(values.destroy_flags, null)
    model_constraints       = try(values.model_constraints, null)
    model_default           = try(values.model_default, null)
    storage_pool            = try(values.storage_pool, null)
  }
}

inputs = merge({
  # Optional inputs
  for k, v in local.optional_inputs :
  k => v
  if v != null
  },
  {
    # Required inputs
    name                 = values.name
    cloud                = values.cloud
    cloud_credential     = values.cloud_credential
    controller_num_units = values.controller_num_units
})
