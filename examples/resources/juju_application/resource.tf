resource "juju_application" "this" {
  name = "my-application"

  model = juju_model.development.name

  charm {
    name     = "hello-kubecon"
    channel  = "edge"
    revision = 14
    series   = "trusty"
  }

  units = 3

  config = {
    external-hostname = "..."
  }
}

resource "juju_application" "placement_example" {
  name  = "placement-example"
  model = juju_model.development.name
  charm {
    name     = "hello-kubecon"
    channel  = "edge"
    revision = 14
    series   = "trusty"
  }

  units     = 3
  placement = "0,1,2"

  config = {
    external-hostname = "..."
  }
}

resource "juju_application" "custom_resources_example" {
  name  = "custom-resource-example"
  model = juju_model.development.name
  charm {
    name     = "hello-kubecon"
    channel  = "edge"
    revision = 14
    series   = "trusty"
  }

  resources = {
    gosherve-image = "gatici/gosherve:1.0"
  }

  units     = 3
  placement = "0,1,2"
}