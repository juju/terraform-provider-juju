provider "juju" {
  version = "~> 0.10.0"

  controller_addresses = "10.225.205.241:17070,10.225.205.242:17070"

  client_id     = "jujuclientid"
  client_secret = "jujuclientsecret"

  ca_certificate = file("~/ca-cert.pem")
}

resource "juju_model" "development" {
  name = "development"

  cloud {
    name   = "aws"
    region = "eu-west-1"
  }
}

resource "juju_application" "wordpress" {
  name = "wordpress"

  model_uuid = juju_model.development.uuid

  charm {
    name = "wordpress"
  }

  units = 3
}

resource "juju_application" "percona-cluster" {
  name = "percona-cluster"

  model_uuid = juju_model.development.uuid

  charm {
    name = "percona-cluster"
  }

  units = 3
}

resource "juju_integration" "wp_to_percona" {
  model_uuid = juju_model.development.uuid

  application {
    name     = juju_application.wordpress.name
    endpoint = "db"
  }

  application {
    name     = juju_application.percona-cluster.name
    endpoint = "server"
  }
}
