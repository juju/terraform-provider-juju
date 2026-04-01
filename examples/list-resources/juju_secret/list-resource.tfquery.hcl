list "juju_secret" "this" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = "<model-uuid>"

    # Optional: filter by secret name
    # name = "my-secret"
  }
}
