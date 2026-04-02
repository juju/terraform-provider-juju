locals {
  # Obtained from `juju show-credentials --client localhost localhost --show-secrets --format yaml`
  lxd_creds = yamldecode(file("~/lxd-credentials.yaml"))
}

resource "juju_controller" "this" {
  name          = "my-controller"
  agent_version = "3.6.14"
  # If using Snap, use the unconfined Juju binary.
  juju_binary    = "/snap/juju/current/bin/juju"
  bootstrap_base = "ubuntu@24.04"

  # Constraints for the provisioned controller machine.
  bootstrap_constraints = {
    "cores"     = "2"
    "mem"       = "4G"
    "root-disk" = "10G"
    "arch"      = "amd64"
  }

  # Here we use Juju's built-in cloud for LXD, but 
  # you can also specify a custom cloud definition.
  cloud = {
    name       = "localhost"
    auth_types = ["certificate"]
    type       = "lxd"
  }

  # Credentials to authenticate with the cloud
  cloud_credential = {
    name      = "test-credential"
    auth_type = "certificate"

    attributes = {
      server-cert = local.lxd_creds.server-cert
      client-key  = local.lxd_creds.client-key
      client-cert = local.lxd_creds.client-cert
    }
  }

  bootstrap_config = {
    "admin-secret" = "test-secret"
  }

  controller_config = {
    "allow-model-access" = "true"
  }

  controller_model_config = {
    "http-proxy"  = "http://proxy.example.com:8080"
    "https-proxy" = "http://proxy.example.com:8080"
  }

  # Optional: If you import a controller, you may need 
  # to ignore changes to certain fields that are not fetched.
  #   lifecycle {
  #     ignore_changes = [
  #       cloud.endpoint,
  #       cloud.region,
  #       cloud_credential.attributes["client-cert"],
  #       cloud_credential.attributes["client-key"]
  #     ]
  #   }
}
