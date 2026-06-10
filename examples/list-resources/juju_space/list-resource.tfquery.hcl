list "juju_space" "this" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = "<model-uuid>"

    # Optional: filter by space name
    # name = "my-space"
  }
}
