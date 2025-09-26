resource "juju_offer" "myoffer" {
  model_uuid       = juju_model.development.uuid
  application_name = juju_application.percona-cluster.name
  endpoints        = ["server"]
}

// an offer can then be used in an cross model integration as below:
resource "juju_integration" "myintegration" {
  model = juju_model.development-destination.name

  application {
    name     = juju_application.wordpress.name
    endpoint = "db"
  }
  application {
    offer_url = juju_offer.myoffer.url
  }
}
