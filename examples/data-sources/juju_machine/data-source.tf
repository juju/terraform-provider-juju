data "juju_machine" "this" {
  model_uuid = juju_model.development.uuid
  machine_id = "2"
}
