resource "juju_application" "this" {
  name = "my-application"

  model = juju_model.development.name

  charm {
    name     = "ubuntu"
    channel  = "edge"
    revision = 24
    series   = "trusty"
  }

  units = 3

  placement = "0,1,2"

  storage = {
    files = "101M"
  }

  config = {
    external-hostname = "..."
  }
}