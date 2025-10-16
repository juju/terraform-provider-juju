# Test file for juju_application data source transformations
resource "juju_model" "test" {
  name = "test-model"
}

# juju_application with resource reference (should be upgraded)
data "juju_application" "resource_ref" {
  name       = "resource_ref"
  model_uuid = juju_model.test.uuid
}

# juju_application with data source reference (should be upgraded)
data "juju_application" "data_ref" {
  name       = "data_ref"
  model_uuid = data.juju_model.existing.uuid
}

# juju_application already using model_uuid (should NOT be upgraded)
data "juju_application" "already_correct" {
  name       = "already_correct"
  model_uuid = juju_model.test.uuid
}

# juju_application with variable reference (should be upgraded)
data "juju_application" "with_variable" {
  name       = "with_variable"
  model_uuid = var.model_name
}
