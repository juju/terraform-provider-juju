# Hello-juju deployment using a postgresql database.
# Check https://charmhub.io/hello-juju for more details
#
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
  model = juju_model.app.name
  charm {
    name = "postgresql"
  }
}

resource "juju_integration" "hello_juju_db" {
  model = juju_model.app.name

  application {
    name     = juju_application.hello_juju.name
    endpoint = "db"
  }

  application {
    name     = juju_application.postgresql.name
    endpoint = "db"
  }
}
