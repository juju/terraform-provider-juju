data "juju_model" "my_model" {
  name = "default"
}

data "juju_subnets" "this" {
  model_uuid = data.juju_model.my_model.uuid
  space_name = "alpha"
}
