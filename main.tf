terraform {
  required_version = ">= 1.14"
  required_providers {
    juju = {
      source  = "juju/juju"
      version = "~> 2.1.1"
    }
  }
}

resource "juju_model" "my_model" {
  name  = "testing"
}

resource "juju_secret" "credentials" {
  model_uuid = juju_model.my_model.uuid
  name       = "my-credentials"

  value = {
    "secret-key"     = "CHANGE_A"
  }

  # Prevent Terraform from overwriting manually populated credentials.
 
}

resource "juju_access_secret" "pgbouncer_credentials" {
  model_uuid = juju_model.my_model.uuid
  secret_id    = juju_secret.credentials.secret_id
  applications = [juju_application.pgbouncer.name]
}

resource "juju_application" "pgbouncer" {
  name        = "pgbouncer"
  model_uuid  = juju_model.my_model.uuid
  trust       = true
  constraints = "arch=amd64 cpu-power=900"

  charm {
    name     = "pgbouncer-k8s"
    channel  = "1/stable"
    revision = 562
  }
}

resource "juju_application" "postgresql" {
  name       = "postgresql"
  model_uuid = juju_model.my_model.uuid
  trust      = true

  charm {
    name     = "postgresql-k8s"
    channel  = "14/stable"
    revision = 925
  }
}

resource "juju_integration" "pgbouncer_postgresql" {
  model_uuid = juju_model.my_model.uuid

  application {
    name = juju_application.pgbouncer.name
  }

  application {
    name = juju_application.postgresql.name
  }
}