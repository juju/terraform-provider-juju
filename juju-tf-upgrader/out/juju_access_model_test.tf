# Test file for juju_access_model resource transformations
data "juju_model" "existing" {
  name = "existing-model"
}

resource "juju_model" "test" {
  name = "test-model"
}

# juju_access_model with resource reference (should be upgraded)
resource "juju_access_model" "access1" {
  access     = "write"
  users      = ["bob", "alice"]
  model_uuid = juju_model.test.uuid
}

# juju_access_model with data source reference (should be upgraded)
resource "juju_access_model" "access2" {
  access     = "read"
  users      = ["charlie"]
  model_uuid = data.juju_model.existing.uuid
}

# juju_access_model already using model_uuid (should NOT be upgraded)
resource "juju_access_model" "already_correct" {
  access     = "admin"
  model_uuid = juju_model.test.uuid
  users      = ["admin"]
}

# juju_access_model with variable reference (should be upgraded)
resource "juju_access_model" "with_variable" {
  access     = "read"
  users      = ["user1"]
  model_uuid = var.model_name
}
