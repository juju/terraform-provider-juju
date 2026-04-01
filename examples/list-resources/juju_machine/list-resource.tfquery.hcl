list "juju_machine" "this" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = "<model-uuid>"

    # Optional: filter by machine name
    # name = "my-machine"
  }
}
