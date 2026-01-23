---
myst:
  html_meta:
    description: "Learn how to add, manage access, update passwords, and remove users in Juju controllers using Terraform Provider."
---

(manage-users)=
# Manage users

> See also: {external+juju:ref}`Juju | User <user>`

## Add a user

To add a user to a controller, in your Terraform plan add a `juju_user` resource, specifying a label, a name, and a password. For example:

```terraform
resource "juju_user" "alex" {
  name = "alex"
  password = "alexsupersecretpassword"

}
```

> See more: [`juju_user` (resource)](../reference/terraform-provider/resources/user)


## Manage a user's access to a controller, cloud, model, offer, role, or group

> See more: {ref}`manage-access-to-a-controller`, {ref}`manage-access-to-a-cloud`, {ref}`manage-access-to-a-model`, {ref}`manage-access-to-an-offer`, {ref}`manage-access-to-a-role`, {ref}`manage-access-to-a-group`

## Manager a user's login details

To set or change a user's password, in your Terraform plan add, in the relevant `juju_user` resource definition, change the `password` attribute to the desired value. For example:

```terraform
resource "juju_user" "alex" {
  name = "alex"
  password = "alexnewsupersecretpassword"

}
```

> See more: [`juju_user`](../reference/terraform-provider/resources/user)

## Remove a user

To remove a user, in your Terraform plan remove its resource definition.

> See more: [`juju_user`](../reference/terraform-provider/resources/user)
