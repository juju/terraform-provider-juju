# Test file for juju_machine resource transformations
data "juju_model" "existing" {
  name = "existing-model"
}

resource "juju_model" "test" {
  name = "test-model"
}

# juju_machine with resource reference (should be upgraded)
resource "juju_machine" "machine1" {
  model       = juju_model.test.name
  base        = "ubuntu@22.04"
  name        = "test_machine"
  constraints = "tags=my-machine-tag"
}

# juju_machine with data source reference (should be upgraded)
resource "juju_machine" "machine2" {
  model       = data.juju_model.existing.name
  base        = "ubuntu@20.04"
  name        = "prod_machine"
  constraints = "cores=4 mem=8G"
}

# juju_machine already using model_uuid (should NOT be upgraded)
resource "juju_machine" "already_correct" {
  model_uuid  = juju_model.test.uuid
  base        = "ubuntu@22.04"
  name        = "correct_machine"
  constraints = "tags=correct-tag"
}

# juju_machine with variable reference (should be upgraded)
resource "juju_machine" "with_variable" {
  model       = var.model_name
  base        = "ubuntu@22.04"
  name        = "var_machine"
  constraints = "cores=2"
}
