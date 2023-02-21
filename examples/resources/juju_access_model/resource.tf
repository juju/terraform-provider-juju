resource "juju_access_model" "this" {
  model  = juju_model.dev.name
  access = "write"
  users  = [juju_user.dev.name, juju_user.qa.name]
}
