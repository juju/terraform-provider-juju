data "juju_model" "my_model" {
  name = "default"
}

data "juju_storage_pool" "my_storage_pool_data_source" {
  name       = "my_storage_pool"
  model_uuid = data.juju_model.my_model.uuid
  # Or:
  model_name  = data.juju_model.my_model.name
  model_owner = data.juju_model.my_model.owner
}
