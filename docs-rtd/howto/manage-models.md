(manage-models)=
# How to manage models

> See also: [Juju | Model](https://canonical-juju.readthedocs-hosted.com/en/latest/user/reference/model/)


## Reference an externally managed model

To reference a model that you've created with Juju tools other than the Terraform Provider for Juju, in your Terraform plan add a data source of the `juju_model` type, specifying the name of the model. For example:

```terraform
data "juju_model" "mymodel" {
  name = "development"
}
```

> See more: [`juju_model` (data source)](https://registry.terraform.io/providers/juju/juju/latest/docs/data-sources/model)

## Add a model

To add a model to the controller specified in the `juju` provider definition, in your Terraform plan create a resource of the `juju_model` type, specifying, at the very least, a name. For example:

```terraform
resource "juju_model" "testmodel" {
  name = "machinetest"
}

```

In the case of a multi-cloud controller, you can specify which cloud you want the model to be associated with by defining a `cloud` block. To specify a model configuration, include a `config` block.


> See more: [`juju_model` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/model)

## Configure a model

> See also: [`juju` | Model configuration](https://juju.is/docs/juju/configuration#heading--model-configuration), [`juju` | List of model configuration keys](https://juju.is/docs/juju/list-of-model-configuration-keys)
>
> See related: [`juju` | How to configure a controller](https://juju.is/docs/juju/manage-controllers#heading--configure-a-controller)

With `terraform-provider-juju` you can only set configuration values, only for a specific model, and only a workload model; for anything else, please use the `juju`  CLI.

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

> See more: [`juju_model` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/model)


## Manage constraints for a model
> See also: [`juju` | Constraint](https://juju.is/docs/juju/constraint)

With `terraform-provider-juju` you can only set constraints -- to view them, please use the `juju` CLI.

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

> See more: [`juju_model` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/model)


## Upgrade a model
> See also: [`juju` | Upgrading things](https://juju.is/docs/juju/upgrading)

To migrate a model to another controller, use the `juju` CLI to perform the migration, then, in your Terraform plan, reconfigure the `juju` provider to point to the destination controller (we recommend the method where you configure the provider using static credentials). You can verify your configuration changes by running `terraform plan` and noticing no change: Terraform merely compares the plan to what it finds in your deployment -- if model migration with `juju` has been successful, it should detect no change.


> See more: {ref}`use-terraform-provider-juju`

(destroy-a-model)=
## Destroy a model

To destroy a model, remove its resource definition from your Terraform plan.

> See more: [`juju_model` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/model)


<br>

<small>**Contributors:** @aflynn, @awnns, @barrettj12, @cderici, @hmlanigan,  @pedroleaoc, @pmatulis, @serdarvural80, @timclicks, @tmihoc</small>
