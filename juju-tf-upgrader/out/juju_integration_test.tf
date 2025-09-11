# Test file for juju_integration resource transformations
resource "juju_model" "integration_test" {
  name = "test-model"
}

# juju_integration that should be upgraded (has model field with juju_model reference)
resource "juju_integration" "should_upgrade" {

  application {
    name     = "app1"
    endpoint = "juju-info"
  }

  application {
    name = "app2"
  }
  model_uuid = juju_model.integration_test.uuid
}

# juju_integration that should NOT be upgraded (no model reference)
resource "juju_integration" "no_model" {
  application {
    name = "app1"
  }

  application {
    name = "app2"
  }
}

# juju_integration that should NOT be upgraded (already using model_uuid)
resource "juju_integration" "already_upgraded" {
  model_uuid = juju_model.integration_test.uuid

  application {
    name     = "app1"
    endpoint = "endpoint"
  }

  application {
    name = "app2"
  }
}

# juju_integration that should be upgraded (model references variable)
resource "juju_integration" "variable_ref" {

  application {
    name = "app1"
  }

  application {
    name = "app2"
  }
  model_uuid = var.model_name
}
