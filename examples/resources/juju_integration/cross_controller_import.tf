provider "juju" {
  # In addition to the default controller, configure the offering one
  offering_controllers = {
    "db-controller" = {
      controller_addresses = "..."
      username             = "..."
      password             = "..."
      ca_certificate       = "..."
    }
  }
}
resource "juju_integration" "this" {
  model_uuid = juju_model.development.uuid
  via        = "10.0.0.0/24,10.0.1.0/24"
  application {
    # Controller name must match the name set up in the `provider`s `offering_controllers` block
    offering_controller = "db-controller"
    offer_url           = "owner/db-model.db"
    endpoint            = "db"
  }
  application {
    name     = juju_application.percona-cluster.name
    endpoint = "server"
  }
}
import {
  to = juju_integration.this
  # UUID obtained through a data source
  id = "${data.juju_model.development.uuid}:percona-cluster:server:wordpress:db"
}
