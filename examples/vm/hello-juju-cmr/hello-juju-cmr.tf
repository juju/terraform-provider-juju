# Hello-juju deployment using a postgresql database.
# Check https://charmhub.io/hello-juju for more details
#
# This plan uses a cross model relation (CMR) between 
# models app and db.
# After the plan is applied you can see the hello-juju
# page by browsing the unit IP. You can find it with:
# `juju status -m app`

terraform {
  required_providers {
    juju = {
      source  = "juju/juju"
      version = "0.4.0"
    }
  }
}

provider "juju" {}

resource "juju_model" "app" {
  name = "app"
  config = {
    logging-config = "<root>=INFO;unit=DEBUG"
  }
}

resource "juju_model" "db" {
  name = "db"
}

resource "juju_application" "hello_juju" {
  name  = "myapp"
  model = juju_model.app.name
  charm {
    name = "hello-juju"
  }

  expose {}
}

resource "juju_application" "postgresql" {
  name  = "database"
  model = juju_model.db.name
  charm {
    name = "postgresql"
  }
}

resource "juju_offer" "postgresql_offer" {
  model            = juju_model.db.name
  application_name = juju_application.postgresql.name
  endpoint         = "db"
}

resource "juju_integration" "postgresql_hello_juju" {
  model = juju_model.app.name
  application {
    name     = juju_application.hello_juju.name
    endpoint = "db"
  }
  application {
    offer_url = juju_offer.postgresql_offer.url
  }
}
