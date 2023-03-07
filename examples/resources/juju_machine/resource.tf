resource "juju_machine" "this_machine" {
  model       = juju_model.development.name
  series      = "focal"
  name        = "this_machine"
  constraints = "tags=my-machine-tag"
}