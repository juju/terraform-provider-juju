# Test file for juju_access_secret resource transformations
data "juju_model" "existing" {
  name = "existing-model"
}

resource "juju_model" "test" {
  name = "test-model"
}

# juju_access_secret with resource reference (should be upgraded)
resource "juju_access_secret" "secret1" {
  access    = "write"
  model     = juju_model.test.name
  secret_id = "secret:abc123"
  users     = ["bob", "alice"]
}

# juju_access_secret with data source reference (should be upgraded)
resource "juju_access_secret" "secret2" {
  access    = "read"
  model     = data.juju_model.existing.name
  secret_id = "secret:def456"
  users     = ["charlie"]
}

# juju_access_secret already using model_uuid (should NOT be upgraded)
resource "juju_access_secret" "already_correct" {
  access     = "admin"
  model_uuid = juju_model.test.uuid
  secret_id  = "secret:ghi789"
  users      = ["admin"]
}

# juju_access_secret with variable reference (should be upgraded)
resource "juju_access_secret" "with_variable" {
  access    = "read"
  model     = var.model_name
  secret_id = "secret:jkl012"
  users     = ["user1"]
}
