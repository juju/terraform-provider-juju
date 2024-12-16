data "juju_jaas_role" "test" {
  name = "role-0"
}

output "role_uuid" {
  value = data.juju_jaas_role.test.uuid
}
