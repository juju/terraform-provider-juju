(manage-offers)=
# How to manage offers

> See also: [`juju` | Offer](https://juju.is/docs/juju/offer)


## Create an offer

> Who: User with [offer `admin` access](https://juju.is/docs/juju/user-permissions#heading--offer-admin).

To create an offer, in your Terraform plan, create a resource of the `juju_offer` type, specifying the offering model and the name of the application and application endpoint from which the offer is created:

```terraform
resource "juju_offer" "percona-cluster" {
  model            = juju_model.development.name
  application_name = juju_application.percona-cluster.name
  endpoint         = "server"
}
```

> See more: [`juju_offer` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/offer)


(integrate-with-an-offer)=
## Integrate with an offer

> Who: User with [offer `consume` access](https://juju.is/docs/juju/user-permissions#heading--offer-consume).

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

> See more: [`juju_integration` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/integration)

## Allow traffic from an integrated offer
> Who: User with [offer `admin` access](https://juju.is/docs/juju/user-permissions#heading--offer-admin).

To allow traffic from an integrated offer, in your Terraform plan, in the resource definition where you define the integration with an offer, use the `via` attribute to specify the list of CIDRs for outbound traffic. For example:



```terraform
resource "juju_integration" "this" {
...
  via   = "10.0.0.0/24,10.0.1.0/24"

# the rest of your integration definition

}

```

> See more: [`juju_integration` > `via`](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/integration#via)


## Remove an offer
> Who: User with [offer `admin` access](https://juju.is/docs/juju/user-permissions#heading--offer-admin).

To remove an offer, in your Terraform plan, remove its resource definition.

> See more: [`juju_offer`](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/offer)


<br>

> <small>**Contributors:** @anvial, @cderici, @hmlanigan, @manadart, @simonrichardson, @tmihoc </small>
