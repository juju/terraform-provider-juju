resource "juju_application" "this" {
  name = "my-application"

  model = juju_model.development.name

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