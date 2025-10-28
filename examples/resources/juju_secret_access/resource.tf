resource "juju_secret" "my-secret" {
  model_uuid = juju_model.development.name
  name       = "my_secret_name"
  value = {
    key1 = "value1"
    key2 = "value2"
  }
  info = "This is the secret"
}

resource "juju_access_secret" "my-secret-access" {
  model_uuid = juju_model.development.uuid
  applications = [
    juju_application.app.name, juju_application.app2.name
  ]
  # Use the secret_id from your secret resource or data source.
  secret_id = juju_secret.my-secret.secret_id
}
