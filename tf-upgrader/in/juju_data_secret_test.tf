# Test file for juju_secret data source transformations
data "juju_model" "existing" {
  name = "existing-model"
}

resource "juju_model" "test" {
  name = "test-model"
}

# juju_secret with resource reference (should be upgraded)
data "juju_secret" "resource_ref" {
  name      = "resource_ref"
  secret_id = "x"
  model     = juju_model.test.name
}

# juju_secret with data source reference (should be upgraded)
data "juju_secret" "data_ref" {
  name      = "data_ref"
  secret_id = "x"
  model     = data.juju_model.existing.name
}

# juju_secret already using model_uuid (should NOT be upgraded)
data "juju_secret" "already_correct" {
  name       = "already_correct"
  secret_id  = "x"
  model_uuid = juju_model.test.uuid
}

# juju_secret with variable reference (should NOT be upgraded)
data "juju_secret" "with_variable" {
  name      = "with_variable"
  secret_id = "x"
  model     = var.model_name
}
