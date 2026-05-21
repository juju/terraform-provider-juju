terraform {
  source = "${get_repo_root()}/modules/juju_bootstrap"
}

locals {
  lxd_creds = yamldecode(file(pathexpand("~/lxd-credentials.yaml")))
}

inputs = {
  name           = "ci-lxd-controller"
  bootstrap_base = "ubuntu@24.04"

  bootstrap_constraints = {
    "cores"     = "2"
    "mem"       = "4G"
    "root-disk" = "4G"
  }

  bootstrap_config = {
    "admin-secret" = "ci-admin-password"
  }

  controller_config = {
    "agent-logfile-max-backups" = "3"
    "audit-log-capture-args"    = "true"
  }

  controller_model_config = {
    "disable-telemetry" = "true"
  }

  cloud = {
    auth_types = ["certificate"]
    name       = "localhost"
    type       = "lxd"
  }

  cloud_credential = {
    auth_type = "certificate"
    name      = "lxd-cred"
    attributes = {
      server-cert = local.lxd_creds["server-cert"]
      client-key  = local.lxd_creds["client-key"]
      client-cert = local.lxd_creds["client-cert"]
    }
  }

  controller_num_units = 1

  destroy_flags = {
    destroy_all_models = true
    destroy_storage    = true
  }
}
