(manage-offers)=
# Manage offers

> See also: {external+juju:ref}`Juju | Offer <offer>`


## Reference an externally managed offer

To reference an offer you've created outside of the current Terraform plan, in your Terraform plan add a data source of the `juju_offer` type, specifying the offer's URL. For example:

```terraform
data "juju_offer" "myoffer" {
  url = "admin/development.mysql"
}
```

> See more: [`juju_offer` (data source)](../reference/terraform-provider/data-sources/offer)

## Create an offer

> Who: User with {external+juju:ref}`offer admin access <user-access-offer-admin>`.


To create an offer, in your Terraform plan, create a resource of the `juju_offer` type, specifying the offering model and the name of the application and application endpoint from which the offer is created:

```terraform
resource "juju_offer" "percona-cluster" {
  model_uuid       = juju_model.development.uuid
  application_name = juju_application.percona-cluster.name
  endpoint         = "server"
}
```

> See more: [`juju_offer` (resource)](../reference/terraform-provider/resources/offer)


(integrate-with-an-offer)=
## Integrate with an offer

> Who: User with {external+juju:ref}`offer consume access <user-access-offer-consume>`.

To integrate with an offer, in your Terraform plan create a `juju_integration` resource as usual by specifying two application blocks and a `lifecycle > replace_triggered_by` block, but for the application representing the offer specify the `offer_url`, and in the `lifecycle` block list triggers only for the regular application (not the offer). For example:

```terraform
resource "juju_integration" "wordpress-db" {
  model = juju_model.development-destination.name

  application {
    name     = juju_application.wordpress.name
    endpoint = "db"
  }

  application {
    offer_url = juju_offer.this.url
  }

  lifecycle {
    replace_triggered_by = [
      juju_application.wordpress.name,
      juju_application.wordpress.model,
      juju_application.wordpress.constraints,
      juju_application.wordpress.placement,
      juju_application.wordpress.charm.name,
    ]
  }

}

```

> See more: [`juju_integration` (resource)](../reference/terraform-provider/resources/integration)

## Allow traffic from an integrated offer
> Who: User with {external+juju:ref}`offer admin access <user-access-offer-admin>`.

To allow traffic from an integrated offer, in your Terraform plan, in the resource definition where you define the integration with an offer, use the `via` attribute to specify the list of CIDRs for outbound traffic. For example:



```terraform
resource "juju_integration" "this" {
...
  via   = "10.0.0.0/24,10.0.1.0/24"

# the rest of your integration definition

}

```

> See more: [`juju_integration` > `via`](../reference/terraform-provider/resources/integration)


(manage-access-to-an-offer)=
## Manage access to an offer

Your offer access management options depend on whether the controller you are applying the Terraform plan to is a regular Juju controller or rather a a Juju controller connected to JIMM -- for the former you can grant access only to a user, but for the latter you can grant access to a user, a service account, a role, or a group.


### For a regular Juju controller
To grant one or more users access to an offer, in your Terraform plan add a `juju_access_offer` resource. You must specify the offer URL and setting the Juju access level to the list of users you want to grant that level. For example:

```terraform
resource "juju_access_offer" "this" {
  offer_url = juju_offer.my_application_offer.url
  consume   = [juju_user.dev.name]
}
```

> See more: [`juju_access_offer`](../reference/terraform-provider/resources/access_offer), [Juju | Offer access levels](https://documentation.ubuntu.com/juju/3.6/reference/user/#valid-access-levels-for-application-offers)


### For a Juju controller added to JIMM
To grant one or more users, service accounts, roles, and/or groups access to a model, in your Terraform plan add a resource type `juju_jaas_access_offer`. You must specify the offer URL, the JAAS offer access level, and the desired list desired users, service accounts, roles, and/or groups. For example:

```terraform
resource "juju_jaas_access_offer" "development" {
  offer_url        = juju_offer.myoffer.url
  access           = "consumer"
  users            = ["foo@domain.com"]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
  roles            = [juju_jaas_role.development.uuid]
  groups           = [juju_jaas_group.development.uuid]
}
```

> See more: [`juju_jaas_access_offer`](../reference/terraform-provider/resources/jaas_access_offer), {external+jaas:ref}`JAAS | Offer access levels <list-of-offer-permissions>`



## Remove an offer
> Who: User with {external+juju:ref}`offer admin access <user-access-offer-admin>`.


To remove an offer, in your Terraform plan, remove its resource definition.

> See more: [`juju_offer`](../reference/terraform-provider/resources/offer)
