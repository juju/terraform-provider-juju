(manage-groups)=
# Manage groups

```{note}
In the Juju ecosystem, groups are supported only when using [JAAS](https://canonical-jaas-documentation.readthedocs-hosted.com/en/latest/).
```

## Reference an externally managed group

To reference a group you've created outside of the current Terraform plan, in your Terraform plan add a data source of the `juju_jaas_group` type, specifying the name of the group. For example:

```terraform
data "juju_jaas_group" "test" {
  name = "group-0"
}
```

> See more: [`juju_jaas_group` (data source)](https://registry.terraform.io/providers/juju/juju/latest/docs/data-sources/jaas_group)


## Add a group

To add a group, in your Terraform plan create a resource of the `juju_jaas_group` type, specifying its name. For example:

```terraform
resource "juju_jaas_group" "development" {
  name = "devops-team"
}
```

> See more: [`juju_jaas_group` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/jaas_group)

(manage-access-to-a-group)=
## Manage access to a group

When using Juju with JAAS, to grant one or more users, groups, and/or service accounts access to a group, in your Terraform plan add a resource type `juju_jaas_access_group`, specifying the group ID, the JAAS group access level, and the list of desired users, groups, and/or service accounts. For example:


```{note}
At present, the only valid JAAS group access level is `member`, so granting an entity access to a group effectively means making them a member of the group.
```

```terraform
resource "juju_jaas_access_group" "development" {
  group_id         = juju_jaas_group.target-group.uuid
  access           = "member"
  users            = ["foo@domain.com"]
  groups           = [juju_jaas_group.development.uuid]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
}
```

> See more: [`juju_jaas_access_group`](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/jaas_access_group), [JAAS | Group access levels](https://canonical-jaas-documentation.readthedocs-hosted.com/en/latest/reference/authorisation_model/#group)

## Manage a group's access to a controller, cloud, model, offer, or group

> See more: {ref}`manage-access-to-a-controller`, {ref}`manage-access-to-a-cloud`, {ref}`manage-access-to-a-model`, {ref}`manage-access-to-an-offer`, {ref}`manage-access-to-a-group`