resource "juju_jaas_group" "test" {
  name = "group-0"
}

data "juju_jaas_group" "test" {
  name = juju_jaas_group.test.name
  // from a separate plan use a string literal
  // name = "group-0"
}

output "group_uuid" {
  value = data.juju_jaas_group.test.uuid
}
