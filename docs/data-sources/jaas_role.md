---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "juju_jaas_role Data Source - terraform-provider-juju"
subcategory: ""
description: |-
  A data source representing a Juju JAAS Role.
---

# juju_jaas_role (Data Source)

A data source representing a Juju JAAS Role.

## Example Usage

```terraform
data "juju_jaas_role" "test" {
  name = "role-0"
}

output "role_uuid" {
  value = data.juju_jaas_role.test.uuid
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) The name of the role.

### Read-Only

- `uuid` (String) The UUID of the role. The UUID is used to reference roles in other resources.
