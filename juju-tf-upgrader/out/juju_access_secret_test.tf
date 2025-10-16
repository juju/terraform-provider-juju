# Test file for juju_access_secret resource transformations
resource "juju_model" "test" {
  name = "test-model"
}

# juju_access_secret with resource reference (should be upgraded)
resource "juju_access_secret" "secret1" {
  access     = "write"
  secret_id  = "secret:abc123"
  users      = ["bob", "alice"]
  model_uuid = juju_model.test.uuid
}

# juju_access_secret with data source reference (should be upgraded)
resource "juju_access_secret" "secret2" {
  access     = "read"
  secret_id  = "secret:def456"
  users      = ["charlie"]
  model_uuid = data.juju_model.existing.uuid
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
  access     = "read"
  secret_id  = "secret:jkl012"
  users      = ["user1"]
  model_uuid = var.model_name
}
