resource "juju_access_model" "this" {
  model_uuid = juju_model.dev.uuid
  access     = "write"
  users      = [juju_user.dev.name, juju_user.qa.name]
}
