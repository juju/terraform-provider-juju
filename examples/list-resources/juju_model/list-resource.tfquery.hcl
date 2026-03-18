list "juju_model" "this" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = "<model-uuid>"
  }
}
