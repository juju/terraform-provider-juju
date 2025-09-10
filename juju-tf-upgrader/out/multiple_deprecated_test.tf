resource "juju_application" "wordpress" {
  name = "wordpress"
  charm {
    name = "wordpress"
  }
  placement  = "0"
  units      = 1
  model_uuid = juju_model.my_model.uuid
  base       = "jammy"
}

resource "juju_model" "my_model" {
  name = "wordpress-model"
}
