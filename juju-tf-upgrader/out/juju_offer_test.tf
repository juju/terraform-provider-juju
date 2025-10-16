# Test file for juju_offer resource transformations
resource "juju_model" "test" {
  name = "test-model"
}

# juju_offer with resource reference (should be upgraded)
resource "juju_offer" "offer1" {
  application_name = "test-app"
  endpoints        = ["sink"]
  model_uuid       = juju_model.test.uuid
}

# juju_offer with data source reference (should be upgraded)
resource "juju_offer" "offer2" {
  application_name = "some-app"
  endpoints        = ["source"]
  model_uuid       = data.juju_model.existing.uuid
}

# juju_offer already using model_uuid (should NOT be upgraded)
resource "juju_offer" "already_correct" {
  model_uuid       = juju_model.test.uuid
  application_name = "correct-app"
  endpoints        = ["endpoint"]
}

# juju_offer with variable reference (should be upgraded)
resource "juju_offer" "with_variable" {
  application_name = "var-app"
  endpoints        = ["db"]
  model_uuid       = var.model_name
}
