resource "juju_controller" "controller" {
  name = "my-controller"
  # ...

  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.juju_enable_ha.controller]
    }
  }
}

action "juju_enable_ha" "controller" {
  config {
    api_addresses = juju_controller.controller.api_addresses
    ca_cert       = juju_controller.controller.ca_cert
    username      = juju_controller.controller.username
    password      = juju_controller.controller.password
    units         = 3
  }
}
