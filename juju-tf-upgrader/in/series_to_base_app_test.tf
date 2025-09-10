resource "juju_application" "wordpress" {
  model = juju_model.my_model.name
  name  = "wordpress"
  charm {
    name = "wordpress"
  }
  series = "jammy"
  units  = 1
}

resource "juju_model" "my_model" {
  name = "wordpress-model"
}
