resource "juju_application" "wordpress" {
  name = "wordpress"
  charm {
    name = "wordpress"
  }
  units      = 1
  model_uuid = juju_model.my_model.uuid
}

resource "juju_model" "my_model" {
  name = "wordpress-model"
}
