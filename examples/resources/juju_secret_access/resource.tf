resource "juju_access_secret" "this" {
  model = juju_model.development.name
  applications = [
    juju_application.app.name, juju_application.app2.name
  ]
  secret_id = juju_secret.that.secret_id
}