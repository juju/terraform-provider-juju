data "juju_model" "my_model" {
  name = "default"
}

data "juju_secret" "my_secret_data_source" {
  name       = "my_secret"
  model_uuid = data.juju_model.my_model.uuid
}
