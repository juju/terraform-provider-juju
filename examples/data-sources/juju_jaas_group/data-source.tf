data "juju_jaas_group" "test" {
  name = "group-0"
}

output "group_uuid" {
  value = data.juju_jaas_group.test.uuid
}
