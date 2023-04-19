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

resource "juju_application" "resource_example" {
  name  = "resource-example"
  model = juju_model.development.name
  charm {
    name     = "hello-kubecon"
    channel  = "edge"
    revision = 19
    series   = "focal"
  }

  resource {
    name      = "gosherve-image"
    oci_image = "jnsgruk/gosherve:latest"
  }
}
