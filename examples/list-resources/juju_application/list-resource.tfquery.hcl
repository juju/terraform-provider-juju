list "juju_application" "this" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = "<model-uuid>"

    # Optional: filter by application name
    # application_name = "my-application"
  }
}
