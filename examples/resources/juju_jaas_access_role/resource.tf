resource "juju_jaas_access_role" "development" {
  role_id          = juju_jaas_role.target-role.uuid
  access           = "assignee"
  users            = ["foo@domain.com"]
  roles            = [juju_jaas_role.development.uuid]
  idp_groups       = ["engineering"]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
}
