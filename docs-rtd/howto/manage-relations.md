(manage-relations)=
# Manage relations

> See also: {external+juju:ref}`Juju | Relation <relation>`

## Add a relation

<!--TODO: Streamline story, e.g.: Suppose you have two applications, `mysql` and `wordpress`. These applications can only be related in one way-->

### Add a same-model relation

To add a same-model relation, create a resource of the `juju_integration` type, give it a label (below, `this`), and in its body add:
- a `model` attribute specifying the name of the model where you want to create the relation;
- two `application` blocks, specifying the names of the applications that you want to integrate (and, if necessary, their endpoints_;
- a `lifecycle` block with the `replace_triggered_by` argument specifying the list of application attributes (always the name, model, constraints, placement, and charm name) for which, if they are changed = destroyed and recreated, the relation must be recreated as well.

```{caution}

**To avoid complications (e.g., race conditions) related to how Terraform works:**

Make sure to always specify resources and data sources by reference rather than directly by name.

For example, for a resource / data source of type `juju_model` with label `development` and name `mymodel`, do not specify it as `mymodel` but rather as `juju_model.development.name` / `data.juju_model.development.name`.


```


```terraform
resource "juju_integration" "this" {
  model_uuid = juju_model.development.uuid
  via   = "10.0.0.0/24,10.0.1.0/24"

  application {
    name     = juju_application.wordpress.name
    endpoint = "db"
  }

  application {
    name     = juju_application.percona-cluster.name
    endpoint = "server"
  }

  # Add any RequiresReplace schema attributes of
  # an application in this integration to ensure
  # it is recreated if one of the applications
  # is Destroyed and Recreated by terraform. E.G.:
  lifecycle {
    replace_triggered_by = [
      juju_application.wordpress.name,
      juju_application.wordpress.model,
      juju_application.wordpress.constraints,
      juju_application.wordpress.placement,
      juju_application.wordpress.charm.name,
      juju_application.percona-cluster.name,
      juju_application.percona-cluster.model,
      juju_application.percona-cluster.constraints,
      juju_application.percona-cluster.placement,
      juju_application.percona-cluster.charm.name,
    ]
  }
}
```

> See more: [`juju_integration` (resource)](../reference/terraform-provider/resources/integration), [Terraform | `lifecycle` > `replace_triggered_by`](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle#replace_triggered_by)



### Add a cross-model relation

In a cross-model relation there is also an 'offering' model and a 'consuming' model. The admin of the 'offering' model 'offers' an application for consumption outside of the model and grants an external user access to it. The user on the 'consuming' model can then find an offer to use, consume the offer, and integrate an application on their model with the 'offer' via the same `integrate` command as in the same-model case (just that the offer must be specified in terms of its offer URL or its consume alias). This creates a local proxy for the offer in the consuming model, and the application is subsequently treated as any other application in the model.

> See more: {ref}`integrate-with-an-offer`



## Remove a relation

To remove a relation, in your Terraform plan, remove its resource definition.

> See more: [`juju_integration` (resource)](../reference/terraform-provider/resources/integration)
