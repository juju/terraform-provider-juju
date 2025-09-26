(manage-secrets)=
# Manage secrets

> See also: {external+juju:ref}`Juju | Secret <secret>`

Charms can use relations to share secrets, such as API keys, a database’s address, credentials and so on. This document demonstrates how to interact with them as a Juju user.

```{caution}

The write operations are only available (a) starting with Juju 3.3 and (b) to model admin users looking to manage user-owned secrets.
```

## Reference an externally managed secret

To reference a user secret you've created outside of the current Terraform plan, in your Terraform plan add a data source of the `juju_secret` type, specifying the name of the secret and its host model. For example:

```terraform
data "juju_secret" "my_secret_data_source" {
  name       = "my_secret"
  model_uuid = data.juju_model.my_model.uuid
}
```

> See more: [`juju_offer` (data source)](../reference/terraform-provider/data-sources/offer)


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

> See more: [`juju_secret` (resource)](../reference/terraform-provider/resources/secret)

## Manage access to a secret

Given a model that contains both your (user) secret and the application(s) that you want to grant access to, to grant the application(s) access to the secret, in your Terraform plan create a resource of the `juju_access_secret` type, specifying the model, the secret ID, and the application(s) that you wish to grant access to. For example:

```
resource "juju_access_secret" "my-secret-access" {
  model_uuid = juju_model.development.uuid

  # Use the secret_id from your secret resource or data source.
  secret_id = juju_secret.my-secret.secret_id

  applications = [
    juju_application.app1.name, juju_application.app2.name
  ]
}

```

> See more: [`juju_access_secret`](../reference/terraform-provider/resources/access_secret)


## Update a secret

> *This feature is opt-in because Juju automatically removing secret content might result in data loss.*


To update a (user) secret, update its resource definition from your Terraform plan.

## Remove a secret

To remove a secret, remove its resource definition from your Terraform plan.

> See more: [`juju_secret` (resource)](../reference/terraform-provider/resources/secret)

