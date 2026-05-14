output "juju_cloud" {
  value = juju_controller.controller.cloud.name
}

output "juju_credentials" {
  value = {
    controller_addresses = juju_controller.controller.api_addresses
    username             = juju_controller.controller.username
    password             = juju_controller.controller.password
    ca_certificate       = juju_controller.controller.ca_cert
  }
  sensitive = true
}
