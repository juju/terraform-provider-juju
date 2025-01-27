(manage-charms)=
# Manage charms

(deploy-a-charm)=
## Deploy a charm 

```{important}

The Terraform Provider for Juju does not support deploying a local charm.

```

To deploy a Charmhub charm, in your Terraform plan add a `juju_application` resource, specifying the target model and the charm:

```terraform
resource "juju_application" "this" {
  model = juju_model.development.name

  charm {
    name = "hello-kubecon"
  }
}
```

> See more: [`juju_application` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/application#schema)


(update-a-charm)=
## Update a charm


To update a charm, in the application's resource definition, in the charm attribute, use a sub-attribute specifying a different revision or channel. For example:

```terraform
resource "juju_application" "this" {
  model = juju_model.development.name

  charm {
    name = "hello-kubecon"
    revision = 19
  }

}
```

> See more: [`juju_application` > `charm` > nested schema ](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/application#nested-schema-for-charm)

## Remove a charm 

As a charm is just the *means* by which (an) application(s) are deployed, there is no way to remove the *charm* / *bundle*. What you *can* do, however, is remove the *application* / *model*.

> See more: {ref}`remove-an-application`, {ref}`destroy-a-model`
