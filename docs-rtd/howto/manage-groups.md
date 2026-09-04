---
myst:
  html_meta:
    description: "Learn how to add, reference, and manage JAAS groups and group membership for users, service accounts, and nested groups."
---

(manage-groups)=
# Manage groups

```{note}
In the Juju ecosystem, groups are supported only when using [JAAS](https://documentation.ubuntu.com/jaas/).
```

```{important}
User-managed groups have been removed in JAAS version 4 and later. Groups are
now authoritatively managed by your identity provider (IdP). If you are using
JAAS 4+, the `juju_jaas_group` resource and data source will return an error and
you should migrate away from managing groups in your Terraform plan. See
{ref}`migrate-away-from-user-managed-groups`.
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

(migrate-away-from-user-managed-groups)=
## Migrate away from user-managed groups (JAAS 4+)

User-managed groups have been removed in JAAS version 4 and later. Groups are
now authoritative in your identity provider (IdP), and membership is managed
through the IdP rather than through Terraform.

If your Terraform plan contains a `juju_jaas_group` resource or data source and
you are targeting a JAAS 4+ controller, Terraform operations will fail with an
error similar to:

```text
Error: Client Error

Unable to add group "devops-team", got error: user-managed groups have been
removed in JAAS version 4+; groups are now managed by the identity provider
(IdP): ...
```

To resolve this, remove the group definitions from both your Terraform state and
your configuration.

### 1. Remove the group from Terraform state

Removing a resource from state stops Terraform from managing it without
attempting to delete it in JAAS (the delete call would fail on JAAS 4+ anyway).

For each `juju_jaas_group` resource in your plan, run:

```shell
terraform state rm juju_jaas_group.<name>
```

For example:

```shell
terraform state rm juju_jaas_group.development
```

If you also have `juju_jaas_access_group` resources that reference removed
groups, remove those from state as well:

```shell
terraform state rm juju_jaas_access_group.<name>
```

Data sources do not need to be removed from state, but they must be removed from
your configuration (see below).

### 2. Remove the group blocks from your configuration

Delete the corresponding `resource "juju_jaas_group"`,
`data "juju_jaas_group"`, and any now-unused `resource "juju_jaas_access_group"`
blocks from your `.tf` files, along with any references to their attributes
(for example `juju_jaas_group.development.uuid`).

### 3. Manage membership through your identity provider

Group membership is now assigned outside of Terraform. Assign users to groups in
your identity provider, and continue to manage group relationships in JAAS using
the `openfga` command as before.

### 4. Verify the plan is clean

Run a plan to confirm there are no remaining references to user-managed groups:

```shell
terraform plan
```

The plan should complete without the groups-removed error and without proposing
changes related to the removed group resources.
