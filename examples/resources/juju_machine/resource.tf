resource "juju_machine" "this_machine" {
  model       = juju_model.development.name
  base        = "ubuntu@22.04"
  name        = "this_machine"
  constraints = "tags=my-machine-tag"
}