(manage-migrations)=

# Manage migrations

The following document shows you what changes, if any, are needed to your Terraform plan when migrating a model(s) between Juju controllers and/or JAAS.

## Migrating to another Juju controller

When a model is migrated to a new Juju controller, no changes are needed. Simply update you plan to connect to the new controller.

The only exception to this scenario is when cross-model relations are involved. If both models involved in a cross-model relation are moved, no changes are necessary. If only one model involved in the relation is moved, see below.

(manage-migration-cross-controller-relations)=

### Cross-controller relations

When a model consumes an offer from another model, this is known as a cross-model relation.
If the providing model is moved, your applications will continue to work but the Juju provider does not currently support the creation of cross-controller relations.

This means that if you modify your plan in a way that causes recreation of the relation, the creation operation will fail. A workaround is to use the Juju CLI to create these cross-controller relation.

## Migrating to JAAS

Migrating models to a JAAS environment requires some updates to your Terraform plan when cross-model relations are involved, even if both models in the relation are migrated.

When a model is migrated to JAAS, the model's name and offer URLs will change. The JAAS documention on [model management](https://documentation.ubuntu.com/jaas/latest/howto/manage-models/) provides more detail.

Our recommended way of resolving your Terraform state for this scenario is described below.

Note that regardless of the order you decide to migrate your models, i.e. providing model first or consuming model first, cross-model application offers are expected to continue working.
There is no recommended order to migrate your models.

While it is recommended to migrate all models involved in a relation to JAAS, it is not a requirement, and migration can be done slowly over the course of days and weeks. See the section on {ref}`cross-controller relations <manage-migration-cross-controller-relations>` for the provider's limitations on cross-controller relations.

### Handling the provider model

When the model providing an application offer is migrated to JAAS, its offer URL changes. Running your Terraform plan against the new controller will attempt to recreate the offer, breaking any applications with existing relations.

To resolve this we suggest removing the resource from Terraform's state and re-importing it using the following commands:

```text
terraform state rm juju_offer.<resource-name>
terraform import juju_offer.<resource-name> <new-offer-url>
```

The new offer URL can be obtained by running `juju show-offer <offer-name>` in the model hosting the offer.

### Handling the consuming model

When the model consuming an application is migrated to JAAS, there may be changes required depending on how your plan is designed.

The following snippet will cause an error after migration:

```terraform
data "juju_offer" "source-offer" {
  url = "admin/source-model.dummy-source"
}

resource "juju_integration" "sink_source_integration" {
  model = juju_model.this.name

  application {
    offer_url = juju_offer.source-offer.url
  }

  application {
    name     = juju_application.sink.name
    endpoint = "source"
  }

}
```

The juju_offer data source will return an error because this offer no longer exists at the same URL. However, the relation itself will continue to work because this URL is only used once, during the creation of the relation.

To resolve this error we suggest one of 2 options,

1. Recreate the relation using the new URL.
2. Change the plan to resemble the example below.

The simplest solution is to recreate the relation. Replace the url in the data source with the new offer URL and allow Terraform to recreate the relation. Note that issue https://github.com/juju/juju/issues/20630 highlights a bug that causes issues for integrations on migrated models.

If the application cannot tolerate any downtime, we suggest modifying the plan to hardcode the offer url into the integration resource and recreate the relation during a maintenance window.

Example:

```terraform
resource "juju_integration" "sink_source_integration" {
  model = juju_model.this.name

  application {
    offer_url = "admin/source-model.dummy-source"
  }

  application {
    name     = juju_application.sink.name
    endpoint = "source"
  }

}
```
