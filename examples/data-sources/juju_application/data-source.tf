data "juju_application" "this" {
  name       = juju_application.this.name
  model_uuid = juju_model.model.uuid
}
