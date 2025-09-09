resource "juju_machine" "machine1" {
  model  = juju_model.my_model.name
  series = "jammy"
}

resource "juju_model" "my_model" {
  name = "machine-model"
}
