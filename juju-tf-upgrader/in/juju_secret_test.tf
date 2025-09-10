# Test file for juju_secret resource transformations
data "juju_model" "existing" {
  name = "existing-model"
}

resource "juju_model" "test" {
  name = "test-model"
}

# juju_secret with resource reference (should be upgraded)
resource "juju_secret" "secret1" {
  model = juju_model.test.name
  name  = "my_secret_name"
  value = {
    key1 = "value1"
    key2 = "value2"
  }
  info = "This is the secret"
}

# juju_secret with data source reference (should be upgraded)
resource "juju_secret" "secret2" {
  model = data.juju_model.existing.name
  name  = "another_secret"
  value = {
    username = "admin"
    password = "secret123"
  }
}

# juju_secret already using model_uuid (should NOT be upgraded)
resource "juju_secret" "already_correct" {
  model_uuid = juju_model.test.uuid
  name       = "correct_secret"
  value = {
    api_key = "abc123"
  }
}

# juju_secret with variable reference (should be upgraded)
resource "juju_secret" "with_variable" {
  model = var.model_name
  name  = "var_secret"
  value = {
    token = "xyz789"
  }
}
