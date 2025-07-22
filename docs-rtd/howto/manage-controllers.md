(manage-controllers)=
# Manage controllers

> See also: {external+juju:ref}`Juju | Controller <controller>`

(manage-access-to-a-controller)=
## Manage access to a controller

```{note}
At present the Terraform Provider for Juju supports controller access management only for Juju controllers added to JIMM.
```

When using Juju with JAAS, to grant access to a Juju controller added to JIMM, in your Terraform plan add a resource type `juju_jaas_access_controller`. Access can be granted to one or more users, service accounts, roles, and/or groups. You must specify the model UUID, the JAAS controller access level, and the desired list of users, service accounts, roles, and/or groups. For example:

```terraform
resource "juju_jaas_access_controller" "development" {
  access           = "administrator"
  users            = ["foo@domain.com"]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
  roles            = [juju_jaas_role.development.uuid]
  groups           = [juju_jaas_group.development.uuid]
}
```

> See more: [`juju_jaas_access_controller`](../reference/terraform-provider/resources/jaas_access_controller), {external+jaas:ref}`JAAS | Controller access levels <list-of-controller-permissions>`