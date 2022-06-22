resource "juju_deployment" "this" {
  name = "foobar" # optional, set to charm name when absent

  model = juju_model.development.uuid # required, model uuid

  charm {
    name     = "hello-kubecon" # required, supports CharmHub charms only
    revision = ""              # optional, default: -1
    channel  = ""              # optional, default: stable
  }

  # The number of instances, default: 1
  units = 3

  config = {
    # Application specific configuration
    external-hostname = "..."
  }
}