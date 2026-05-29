resource "juju_model" "development" {
  name = "development"
}

resource "juju_space" "development" {
  model_uuid = juju_model.development.uuid
  name       = "development"
}
