# Test file for juju_model data source transformations
resource "juju_model" "test" {
  name = "test-model"
}

data "juju_model" "test" {
  uuid = juju_model.test.uuid
}
