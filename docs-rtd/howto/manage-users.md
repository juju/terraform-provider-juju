(manage-users)=
# Manage users

> See also: [Juju | User](https://canonical-juju.readthedocs-hosted.com/en/latest/user/reference/user/)

## Add a user

To add a user to a controller, in your Terraform plan add a `juju_user` resource, specifying a label, a name, and a password. For example:

```terraform
resource "juju_user" "alex" {
  name = "alex"
  password = "alexsupersecretpassword"

}
```

> See more: [`juju_user` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/user)


## Manage a user's access to a controller, cloud, model, offer, or group

> See more: {ref}`manage-access-to-a-controller`, {ref}`manage-access-to-a-cloud`, {ref}`manage-access-to-a-model`, {ref}`manage-access-to-an-offer`, {ref}`manage-access-to-a-group`

## Manager a user's login details

To set or change a user's password, in your Terraform plan add, in the relevant `juju_user` resource definition, change the `password` attribute to the desired value. For example:

```terraform
resource "juju_user" "alex" {
  name = "alex"
  password = "alexnewsupersecretpassword"

}
```

> See more: [`juju_user`](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/user#password)

## Remove a user

To remove a user, in your Terraform plan remove its resource definition.

> See more: [`juju_user` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/user)


<br>

> <small>**Contributors:** @cderici, @hmlanigan, @pedroleaoc, @pmatulis, @timclicks, @tmihoc </small>
