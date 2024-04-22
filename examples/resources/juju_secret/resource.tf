resource "juju_secret" "this" {
  model = juju_model.development.name
  name  = "this_secret_name"
  value = {
    key1 = "value1"
    key2 = "value2"
  }
  info = "This is the secret"
}