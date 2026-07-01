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

# Write-only secret value. The value is supplied via value_wo, which is never
# stored in Terraform state. Bumping value_wo_version triggers an update of the
# secret value. This requires Terraform >= 1.11 and pairs well with ephemeral
# resources/values.
resource "juju_secret" "my-wo-secret" {
  model_uuid = juju_model.development.uuid
  name       = "my_wo_secret_name"
  value_wo = {
    key1 = "value1"
    key2 = "value2"
  }
  value_wo_version = 1
  info             = "This is a write-only secret"
}
