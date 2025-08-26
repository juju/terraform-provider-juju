resource "juju_secret" "my-secret" {
  model_uuid = juju_model.development.uuid
  name       = "my_secret_name"
  value = {
    key1 = "value1"
    key2 = "value2"
  }
  info = "This is the secret"
}

resource "juju_application" "my-application" {
  #
  config = {
    # Reference my-secret within the plan by using the secret_id
    secret = juju_secret.my-secret.secret_id
  }
  #
}
