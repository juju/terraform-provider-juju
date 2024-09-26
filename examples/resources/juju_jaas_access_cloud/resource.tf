resource "juju_jaas_access_cloud" "development" {
  cloud_name       = "aws"
  access           = "can_addmodel"
  users            = ["foo@domain.com"]
  groups           = [juju_jaas_group.development.uuid]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
}
