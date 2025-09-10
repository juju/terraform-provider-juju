# Test file for juju_offer resource transformations
data "juju_model" "existing" {
  name = "existing-model"
}

resource "juju_model" "test" {
  name = "test-model"
}

# juju_offer with resource reference (should be upgraded)
resource "juju_offer" "offer1" {
  model            = juju_model.test.name
  application_name = "test-app"
  endpoints        = ["sink"]
}

# juju_offer with data source reference (should be upgraded)
resource "juju_offer" "offer2" {
  model            = data.juju_model.existing.name
  application_name = "some-app"
  endpoints        = ["source"]
}

# juju_offer already using model_uuid (should NOT be upgraded)
resource "juju_offer" "already_correct" {
  model_uuid       = juju_model.test.uuid
  application_name = "correct-app"
  endpoints        = ["endpoint"]
}

# juju_offer with variable reference (should be upgraded)
resource "juju_offer" "with_variable" {
  model            = var.model_name
  application_name = "var-app"
  endpoints        = ["db"]
}
