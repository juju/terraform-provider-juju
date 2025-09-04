# Test file for juju_machine data source transformations
data "juju_model" "existing" {
  name = "existing-model"
}

resource "juju_model" "test" {
  name = "test-model"
}

# juju_machine with resource reference (should be upgraded)
data "juju_machine" "resource_ref" {
  machine_id = "resource_ref"
  model      = juju_model.test.name
}

# juju_machine with data source reference (should be upgraded)
data "juju_machine" "data_ref" {
  machine_id  = "data_ref"
  model       = data.juju_model.existing.name
}

# juju_machine already using model_uuid (should NOT be upgraded)
data "juju_machine" "already_correct" {
  machine_id = "already_correct"
  model_uuid = juju_model.test.uuid
}

# juju_machine with variable reference (should NOT be upgraded)
data "juju_machine" "with_variable" {
  machine_id = "with_variable"
  model      = var.model_name
}
