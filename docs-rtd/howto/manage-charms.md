(manage-charms)=
# Manage charms

> See also: {external+juju:ref}`Juju | Charm <charm>`

(deploy-a-charm)=
## Deploy a charm

The Terraform Provider for Juju does not support deploying a local charm.

To deploy a Charmhub charm, in your Terraform plan add a `juju_application` resource, specifying the target model and the charm you want to deploy:

```terraform
resource "juju_application" "this" {
  model_uuid = juju_model.development.uuid

  charm {
    name = "hello-kubecon"
  }
}
```

You can also specify a charm channel and revision. For example:


```terraform
resource "juju_application" "this" {
    model = <model>
    charm {
        name     = "<charm-name>"
        channel  = "<channel-name>"
        revision = "<revision-number>"
    }
}
```

This works as follows:

- If both `channel` and `revision` are specified (recommended for reproducibility), the Terraform provider will deploy the requested revision.
- If only `channel` is specified, the provider will deploy the latest revision available in that channel. The charm will not be refreshed on subsequent `terraform apply` runs.
- If only `revision` is specified, the provider will try to deploy that revision from the default channel (as set for the charm on Charmhub); if not available, the result will be an error.
- If neither field is specified, the provider will deploy the latest revision from the default channel (as set for the charm on Charmhub). The charm will not be refreshed on subsequent `terraform apply` runs.

If the charm has any resources, and your Terraform plan does not specify them explicitly, resources will come from the tip of the specified or inferred channel.

> See more: [`juju_application` (resource)](../reference/terraform-provider//resources/application)


(update-a-charm)=
## Update a charm

To update a charm, in the application's resource definition, in the charm attribute, use a sub-attribute specifying a different revision or channel. For example:

```terraform
resource "juju_application" "this" {
  model_uuid = juju_model.development.uuid

  charm {
    name = "hello-kubecon"
    revision = 19
  }

}
```

The Terraform provider does not support refreshing the charm when the revision is not specified. When unset, the revision number is determined during application creation. If you wish to keep the revision unset, you can refresh the application manually using the `juju` CLI. However, note that setting both `channel` and `revision` makes for a more reproducible deployment.

When the charm is changed, its resources will also be updated unless pinned.

> See more: [`juju_application` > `charm` > nested schema ](../reference/terraform-provider/resources/application)

## Remove a charm

As a charm is just the *means* by which (an) application(s) are deployed, there is no way to remove the *charm* / *bundle*. What you *can* do, however, is remove the *application* / *model*.

> See more: {ref}`remove-an-application`, {ref}`destroy-a-model`
