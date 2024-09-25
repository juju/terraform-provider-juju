resource "juju_jaas_access_service_account" "development" {
  service_account_id = "Client-ID"
  access             = "administrator"
  users              = ["foo@domain.com"]
  groups             = [juju_jaas_group.development.uuid]
  service_accounts   = ["Client-ID-1", "Client-ID-2"]
}
