(manage-secrets)=
# How to manage secrets

> See also: [`juju` | Secret](https://juju.is/docs/juju/secret)

Charms can use relations to share secrets, such as API keys, a database’s address, credentials and so on. This document demonstrates how to interact with them as a Juju user. 

```{caution}

The write operations are only available (a) starting with Juju 3.3 and (b) to model admin users looking to manage user-owned secrets. 
```

## Add a secret


To add a (user) secret on the controller specified in the juju provider definition, in your Terraform plan create a resource of the `juju_secret` type, specifying, at the very least, a model, the name of the secret, a values map and, optionally, an info field. For example:

```terraform
resource "juju_secret" "my-secret" {
  model = juju_model.development.name
  name  = "my_secret_name"
  value = {
    key1 = "value1"
    key2 = "value2"
  }
  info = "<description of the secret>"
}
```

> See more: [`juju_secret` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/secret)

## Grant access to a secret


Given a model that contains both your (user) secret and the application(s) that you want to grant access to, to grant the application(s) access to the secret, in your Terraform plan create a resource of the `juju_access_secret` type, specifying the model, the secret ID, and the application(s) that you wish to grant access to. For example:

```
resource "juju_access_secret" "my-secret-access" {
  model = juju_model.development.name

  # Use the secret_id from your secret resource or data source.
  secret_id = juju_secret.my-secret.secret_id

  applications = [
    juju_application.app.name, juju_application.app2.name
  ]
}

```

> See more: [`juju_access_secret` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/access_secret)


## Update a secret

> *This feature is opt-in because Juju automatically removing secret content might result in data loss.*


To update a (user) secret, update its resource definition from your Terraform plan.

## Remove a secret

To remove a secret, remove its resource definition from your Terraform plan.

> See more: [`juju_secret` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/secret)

<br>

> <small>Contributors: @anvial, @cderici, @kelvin.liu , @tmihoc, @tony-meyer , @wallyworld </small>
