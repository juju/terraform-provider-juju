(manage-groups)=
# Manage groups

```{note}
In the Juju ecosystem, groups are supported only when using [JAAS](https://documentation.ubuntu.com/jaas/).
```

## Reference an externally managed group

To reference a group you've created outside of the current Terraform plan, in your Terraform plan add a data source of the `juju_jaas_group` type, specifying the name of the group. For example:

```terraform
data "juju_jaas_group" "test" {
  name = "group-0"
}
```

> See more: [`juju_jaas_group` (data source)](../reference/terraform-provider/data-sources/jaas_group)


## Add a group

To add a group, in your Terraform plan create a resource of the `juju_jaas_group` type, specifying its name. For example:

```terraform
resource "juju_jaas_group" "development" {
  name = "devops-team"
}
```

> See more: [`juju_jaas_group` (resource)](../reference/terraform-provider/resources/jaas_group)

(manage-access-to-a-group)=
## Manage access to a group

When using Juju with JAAS, to grant access to a group, in your Terraform plan add a resource type `juju_jaas_access_group`. Access can be granted to one or more users, service accounts, and/or groups. The resource must include the group ID, the JAAS group access level, and the list of desired users, service accounts, and/or groups. For example:

```{note}
At present, the only valid JAAS group access level is `member`, so granting an entity access to a group effectively means making them a member of the group.
```

```terraform
resource "juju_jaas_access_group" "development" {
  group_id         = juju_jaas_group.target-group.uuid
  access           = "member"
  users            = ["foo@domain.com"]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
  groups           = [juju_jaas_group.development.uuid]
}
```

> See more: [`juju_jaas_access_group`](../reference/terraform-provider/resources/jaas_access_group), {external+jaas:ref}`JAAS | Group access levels <list-of-group-permissions>`

## Manage a group's access to a controller, cloud, model, offer, role, or group

> See more: {ref}`manage-access-to-a-controller`, {ref}`manage-access-to-a-cloud`, {ref}`manage-access-to-a-model`, {ref}`manage-access-to-an-offer`, {ref}`manage-access-to-a-role`, {ref}`manage-access-to-a-group`