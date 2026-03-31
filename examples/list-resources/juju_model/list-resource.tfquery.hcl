list "juju_model" "this" {
  provider         = juju
  include_resource = true

  config {
    # Optional: filter by model uuid
    # model_uuid = "<model-uuid>"
  }
}
