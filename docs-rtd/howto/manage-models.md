(manage-models)=
# Manage models

> See also: {external+juju:ref}`Juju | Model <model>`


## Reference an externally managed model

To reference a model that you've created outside of the current Terraform plan, in your Terraform plan add a data source of the `juju_model` type, specifying the name of the model. For example:

```terraform
data "juju_model" "mymodel" {
  name = "development"
}
```

> See more: [`juju_model` (data source)](../reference/terraform-provider/data-sources/model)

## Add a model

To add a model to the controller specified in the `juju` provider definition, in your Terraform plan create a resource of the `juju_model` type, specifying, at the very least, a name. For example:

```terraform
resource "juju_model" "testmodel" {
  name = "machinetest"
}

```

In the case of a multi-cloud controller, you can specify which cloud you want the model to be associated with by defining a `cloud` block. To specify a model configuration, include a `config` block.


> See more: [`juju_model` (resource)](../reference/terraform-provider/resources/model)

## Configure a model

> See also: {external+juju:ref}`Juju | Model configuration <model-configuration>`, {external+juju:ref}`Juju | List of model configuration keys <list-of-model-configuration-keys>`
>
> See related: {external+juju:ref}`Juju | Configure a controller <configure-a-controller>`

With the Terraform Provider for Juju you can only set configuration values, only for a specific model, and only a workload model; for anything else, please use the `juju`  CLI.

To configure a specific workload model, in your Terraform plan, in the model's resource definition, specify a `config` block, listing all the key=value pairs you want to set. For example:

```terraform
resource "juju_model" "this" {
  name = "development"

  cloud {
    name   = "aws"
    region = "eu-west-1"
  }

  config = {
    logging-config              = "<root>=INFO"
    development                 = true
    no-proxy                    = "jujucharms.com"
    update-status-hook-interval = "5m"
  }
}
```

> See more: [`juju_model` (resource)](../reference/terraform-provider/resources/model)


## Manage constraints for a model
> See also: {external+juju:ref}`Juju | Constraint <constraint>`

With the Terraform Provider for Juju you can only set constraints -- to view them, please use the `juju` CLI.

To set constraints for a model, in your Terraform, in the model's resource definition, specify the `constraints` attribute (value is a quotes-enclosed space-separated list of key=value pairs). For example:

```terraform
resource "juju_model" "this" {
  name = "development"

  cloud {
    name   = "aws"
    region = "eu-west-1"
  }

  constraints = "cores=4 mem=16G"
}
```

> See more: [`juju_model` (resource)](../reference/terraform-provider/resources/model)


(manage-access-to-a-model)=

## Manage annotations for a model
To set annotations for a model, in your Terraform, in the model's resource definition, specify an `annotations` block, listing all the key=value pairs you want to set. For example:

```terraform
resource "juju_model" "testmodel" {
  name = "model"

  annotations = {
	  test = "test"
  }
}
```

> See more: [`juju_model` (resource)](../reference/terraform-provider/resources/model)

## Manage access to a model

Your model access management options depend on whether the controller you are applying the Terraform plan to is a regular Juju controller or rather a Juju controller added to JIMM -- for the former you can grant access only to a user, but for the latter you can grant access to a user, a service account, a role, or a group.


### For a regular Juju controller
To grant one or more users access to a model, in your Terraform plan add a `juju_access_model` resource. You must specify the model, the Juju access level, and the user(s) to which you want to grant access. For example:

```terraform
resource "juju_access_model" "this" {
  model  = juju_model.dev.name
  access = "write"
  users  = [juju_user.dev.name, juju_user.qa.name]
}
```

> See more: [`juju_access_model`](../reference/terraform-provider/resources/access_model), [Juju | Model access levels](https://documentation.ubuntu.com/juju/3.6/reference/user/#valid-access-levels-for-models)


### For a Juju controller added to JIMM
To grant one or more users, service accounts, roles, and/or groups access to a model, in your Terraform plan add a resource type `juju_jaas_access_model`. You must specify the model UUID, the JAAS model access level, and the desired list of users, service accounts, roles, and/or groups. For example:

```terraform
resource "juju_jaas_access_model" "development" {
  model_uuid       = juju_model.development.uuid
  access           = "administrator"
  users            = ["foo@domain.com"]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
  roles            = [juju_jaas_role.development.uuid]
  groups           = [juju_jaas_group.development.uuid]
}

```

> See more: [`juju_jaas_access_model`](../reference/terraform-provider/resources/jaas_access_model), {external+jaas:ref}`JAAS | Model access levels <list-of-model-permissions>`

## Upgrade a model
> See also: {external+juju:ref}`Juju | Upgrading things <upgrading-things>`

To migrate a model to another controller, use the `juju` CLI to perform the migration, then, in your Terraform plan, reconfigure the `juju` provider to point to the destination controller (we recommend the method where you configure the provider using static credentials). You can verify your configuration changes by running `terraform plan` and noticing no change: Terraform merely compares the plan to what it finds in your deployment -- if model migration with `juju` has been successful, it should detect no change.


> See more: {ref}`use-the-terraform-provider-for-juju`

(destroy-a-model)=
## Destroy a model

To destroy a model, remove its resource definition from your Terraform plan.

> See more: [`juju_model` (resource)](../reference/terraform-provider/resources/model)

