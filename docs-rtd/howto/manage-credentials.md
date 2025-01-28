(manage-credentials)=
# Manage credentials

> See also: [Juju | Credential](https://juju.is/docs/juju/credential)


## Define a credential

To define a credential, in your Terraform plan create a resource of the `juju_credential` type, specifying the credential's name, cloud, authentication type, and the attributes associated with the authentication type.


```terraform
resource "juju_credential" "this" {
  name = "creddev"

  cloud {
    name = "localhost"
  }

  auth_type = "certificate"

  attributes = {
    client-cert    = "/srv/cert.crt"
    client-key     = "/srv/cert.key"
    trust-password = "S0m3P@$$w0rd"
  }
}
```

> See more: [`juju_credential` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/credential)

## Add a credential to a model
> Who: User with controller `superuser` or model `admin` access.


To add a controller credential to a model, in your Terraform plan, specify it as an attribute to the model definition. For example:

```terraform
resource "juju_model" "this" {
  name = "development"

  cloud {
    name   = "aws"
    region = "eu-west-1"
  }

  credential = "juju_credential.<credential label>.name"
}
```


## Update a credential

To update a credential, in your Terraform plan, update its resource definition.

> See more: [Resource `juju_credential`](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/credential)

## Remove a credential

To remove a credential, remove its resource definition from your Terraform plan.

> See more: [`juju_credential`](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/credential)
