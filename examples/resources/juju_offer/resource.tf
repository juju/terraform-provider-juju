resource "juju_offer" "this" {
  model            = juju_model.development.name
  application_name = juju_application.percona-cluster.name
  endpoint         = server
}

// an offer can then be used in an integration as below:
resource "juju_integration" "this" {
  model = juju_model.development-destination.name

  application {
    name     = juju_application.wordpress.name
    endpoint = "db"
  }

  application {
    offer_url = juju_offer.this.url
  }
}
