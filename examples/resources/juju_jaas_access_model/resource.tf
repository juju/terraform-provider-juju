resource "juju_jaas_access_model" "development" {
  model_uuid       = juju_model.development.uuid
  access           = "administrator"
  users            = ["foo@domain.com"]
  groups           = [juju_jaas_group.development.uuid]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
}
