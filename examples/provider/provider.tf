terraform {
  required_providers {
    juju = {
      version = "~> 0.12.0"
      source  = "juju/juju"
    }
  }
}

provider "juju" {}

resource "juju_model" "development" {
  name = "development"

  cloud {
    name   = "aws"
    region = "eu-west-1"
  }
}

resource "juju_application" "wordpress" {
  name = "wordpress"

  model = juju_model.development.name

  charm {
    name = "wordpress"
  }

  units = 3
}

resource "juju_application" "percona-cluster" {
  name = "percona-cluster"

  model = juju_model.development.name

  charm {
    name = "percona-cluster"
  }

  units = 3
}

resource "juju_integration" "wp_to_percona" {
  model = juju_model.development.name

  application {
    name     = juju_application.wordpress.name
    endpoint = "db"
  }

  application {
    name     = juju_application.percona-cluster.name
    endpoint = "server"
  }
}
