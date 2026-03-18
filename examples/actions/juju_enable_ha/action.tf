resource "juju_controller" "this" {
  name = "my-controller"
  # ...
}

action "juju_enable_ha" "this" {
  api_addresses = juju_controller.this.api_addresses
  ca_cert       = juju_controller.this.ca_cert
  username      = juju_controller.this.username
  password      = juju_controller.this.password
  units         = 3
}
