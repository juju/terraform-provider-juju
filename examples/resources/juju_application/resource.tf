resource "juju_application" "this" {
  name = "my-application"

  model_uuid = juju_model.development.uuid

  charm {
    name     = "ubuntu"
    channel  = "edge"
    revision = 24
    series   = "trusty"
  }

  resources = {
    gosherve-image = "gatici/gosherve:1.0"
  }

  units = 3

  placement = "0,1,2"

  storage_directives = {
    files = "101M"
  }

  config = {
    external-hostname = "..."
  }
}

# K8s application with resource from private registry
resource "juju_application" "this" {
  model_uuid = juju_model.this.uuid
  name       = "test-app"
  charm {
    name    = "coredns"
    channel = "latest/stable"
  }
  trust = true
  expose {}
  registry_credentials = {
    "ghcr.io/canonical" = {
      username = "username"
      password = "password"
    }
  }
  resources = {
    "coredns-image" : "ghcr.io/canonical/test:dfb5e3fa84d9476c492c8693d7b2417c0de8742f"
  }
}
