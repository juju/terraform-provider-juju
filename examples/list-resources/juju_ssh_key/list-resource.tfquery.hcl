list "juju_ssh_key" "this" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = "<model-uuid>"
  }
}
