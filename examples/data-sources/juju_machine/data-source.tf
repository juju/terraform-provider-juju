data "juju_machine" "this" {
  model      = juju_model.development.name
  machine_id = "2"
}
