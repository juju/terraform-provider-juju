(manage-users)=
# How to manage users

> See also: [`juju` | User](https://juju.is/docs/juju/user)

## Add a user

To add a user to a controller, in your Terraform plan add a `juju_user` resource, specifying a label, a name, and a password. For example:

```terraform
resource "juju_user" "alex" {
  name = "alex"
  password = "alexsupersecretpassword"

}
``` 

> See more: [`juju_user` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/user)


## Manage a user's access level
> See also: [`juju` | User access levels](https://juju.is/docs/juju/user-permissions)

With `terraform-provider-juju` you can manage user access only at the model level; for anything else, please use the `juju` CLI.

To grant a user access to a model, in your Terraform plan add a `juju_access_model` resource, specifying the model, the access level, and the user(s) to which you want to grant access. For example:

```terraform
resource "juju_access_model" "this" {
  model  = juju_model.dev.name
  access = "write"
  users  = [juju_user.dev.name, juju_user.qa.name]
}
```

> See more: [`juju_access_model`](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/access_model)

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
