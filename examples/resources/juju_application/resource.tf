# An application with config, multiple units and constraints
resource "juju_application" "this" {
  name = "my-application"

  model_uuid = juju_model.development.uuid

  charm {
    name     = "ubuntu"
    channel  = "latest/stable"
    revision = 24
    base     = "ubuntu@24.04"
  }

  units = 3

  constraints = "mem=4G cores=2"

  config = {
    external-hostname = "..."
  }
}

# An application with storage directives
resource "juju_application" "this" {
  name = "my-application"

  model_uuid = juju_model.development.uuid

  charm {
    name    = "postgresql"
    channel = "14/stable"
    base    = "ubuntu@22.04"
  }

  storage_directives = {
    "pgdata" = "4G" # 4 gigabytes of storage for pgdata using the model's default storage pool
    # or
    "pgdata" = "2,4G" # 2 instances of 4 gigabytes of storage for pgdata using the model's default storage pool
    # or
    "pgdata" = "ebs,2,4G" # 2 instances of 4 gigabytes of storage for pgdata on the ebs storage pool
  }
}

# An application deployed to specific machines
# This example creates a set of machines and deploys an application to those machines.
resource "juju_machine" "all_machines" {
  count      = 5
  model_uuid = juju_model.model.uuid
  base       = "ubuntu@22.04"
  name       = "machine_${count.index}"

  # The following lifecycle directive instructs Terraform to create 
  # new machines before destroying existing ones.
  lifecycle {
    create_before_destroy = true
  }
}

resource "juju_application" "testapp" {
  name       = "juju-qa-test"
  model_uuid = juju_model.model.uuid


  machines = toset(juju_machine.all_machines[*].machine_id)

  charm {
    name    = "ubuntu"
    channel = "latest/stable"
    base    = "ubuntu@22.04"
  }
}

# K8s application with an OCI image resource from a private registry
resource "juju_application" "this" {
  name = "test-app"

  model_uuid = juju_model.this.uuid

  charm {
    name    = "coredns"
    channel = "latest/stable"
  }

  trust = true
  expose {}

  registry_credentials = {
    "ghcr.io/canonical" = {
      username = "username"
      password = "password"
    }
  }

  resources = {
    "coredns-image" : "ghcr.io/canonical/test:dfb5e3fa84d9476c492c8693d7b2417c0de8742f"
  }
}
