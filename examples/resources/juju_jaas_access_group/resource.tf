resource "juju_jaas_access_group" "development" {
  group_uuid       = juju_jaas_group.target-group.uuid
  access           = "member"
  users            = ["foo@domain.com"]
  groups           = [juju_jaas_group.development.uuid]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
}
