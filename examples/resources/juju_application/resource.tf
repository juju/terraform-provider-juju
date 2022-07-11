resource "juju_application" "this" {
  name = "foobar" # optional, set to charm name when absent

  model = juju_model.development.name # required, model name

  charm {
    name     = "hello-kubecon" # required, supports CharmHub charms only
    channel  = "edge"          # optional, specified as <track>/<risk>/<branch>, default: latest/stable
    revision = 14              # optional, default: -1
    series   = "trusty"        # optional
  }

  # The number of instances, default: 1
  units = 3

  config = {
    # Application specific configuration
    external-hostname = "..."
  }
}