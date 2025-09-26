resource "juju_machine" "machine1" {
  model_uuid = juju_model.my_model.uuid
  base       = "jammy"
}

resource "juju_model" "my_model" {
  name = "machine-model"
}
