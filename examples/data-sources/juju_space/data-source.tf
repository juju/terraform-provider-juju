data "juju_model" "my_model" {
  name = "default"
}

data "juju_space" "my_space_data_source" {
  model_uuid = data.juju_model.my_model.uuid
  name       = "my-space"
}
