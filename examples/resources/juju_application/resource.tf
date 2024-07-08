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

<<<<<<< HEAD
  placement = "0,1,2"

  storage_directives = {
    files = "101M"
  }

=======
>>>>>>> 17accba (Removing additional example from resource.tf)
  config = {
    external-hostname = "..."
  }
}

<<<<<<< HEAD
resource "juju_application" "custom_resources_example" {
  name  = "custom-resource-example"
=======
resource "juju_application" "placement_example" {
  name  = "placement-example"
>>>>>>> 17accba (Removing additional example from resource.tf)
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

  config = {
    external-hostname = "..."
  }
}