resource "juju_model" "development" {
  name = "development"
}

resource "juju_space" "development" {
  model_uuid = juju_model.development.uuid
  name       = "development"
}

resource "juju_subnet" "development" {
  model_uuid = juju_model.development.uuid
  cidr       = "10.0.0.0/24"
  space      = juju_space.development.name
}
