resource "juju_controller" "this" {
  name = "my-controller"

  # See the controller resource example for a full example 
  # of the juju_controller resource configuration.
}

resource "juju_jaas_controller" "jaas" {
  name = juju_controller.this.name
  uuid = juju_controller.this.controller_uuid

  api_addresses  = juju_controller.this.api_addresses
  ca_certificate = juju_controller.this.ca_cert

  username = juju_controller.this.username
  password = juju_controller.this.password

  tls_hostname = "juju-apiserver"
}
