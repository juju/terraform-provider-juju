(install-and-manage-terraform-provider-juju)=
# How to install and manage `terraform-provider-juju`

## Install `terraform-provider-juju`

To install `terraform-provider-juju` on Linux, macOS, or Windows, you need to install the `terraform` CLI. 

> See more: [Hashicorp | Install Terraform](https://developer.hashicorp.com/terraform/install)

For example, on a Linux that supports snaps:

```text
sudo snap install terraform
```

(use-terraform-provider-juju)=
## Use `terraform-provider-juju`


Once you've installed the `terraform` CLI, to start using it:

1. **Require the `juju` provider.** In your Terraform plan, under `required_providers`, specify the `juju` provider:

    ```terraform
    terraform {
      required_providers {
        juju = {
          version = "~> 0.10.0"
          source  = "juju/juju"
        }
      }
    }
    ```

2. **Configure the provider to use a pre-existing controller.** There are 3 ways you can do this: using static credentials, using environment variables, or using the `juju` client.

    ```{note}
    For all methods: To view your controller's details, run `juju show-controller --show-password`.

    ```

    ::::{dropdown} Configure the provider using static credentials

    In your Terraform plan, in your provider specification, use the various keywords to provide your controller information statically:

    ```terraform
    provider "juju" {
      controller_addresses = "10.225.205.241:17070,10.225.205.242:17070"
      username = "jujuuser"
      password = "password1"
      ca_certificate = file("~/ca-cert.pem")
    }
    ```

    > See more: [Terraform | `juju` provider](https://registry.terraform.io/providers/juju/juju/latest/docs)

    ::::

    ::::{dropdown} Configure the provider using environment variables

    In your Terraform plan, leave the `provider` specification empty:

    ```terraform
    provider "juju" {}
    ``` 

    Then, in a terminal, export the controller environment variables with your controller's values. For example:

    ```bash
    export JUJU_CONTROLLER_ADDRESSES="10.225.205.241:17070,10.225.205.242:17070"
    export JUJU_USERNAME="jujuuser"
    export JUJU_PASSWORD="password1"
    export JUJU_CA_CERT=file("~/ca-cert.pem")
    ```

    ::::


    ::::{dropdown} Configure the provider using the juju CLI

    In your Terraform plan, leave the `provider` specification empty:

    ```terraform
    provider "juju" {}
    ```

    Then, in a terminal, use the `juju` client to switch to the desired controller: `juju switch <controller>`. Your Terraform plan will be interpreted relative to that controller.

    ::::

3. Build your deployment.

    > See more: [How-to guides](../howto/index)

4. Once you're done, in a terminal, run:

    a. (just the first time) `terraform init` to initialise your project;

    b. `terraform plan` to stage the changes; and

    c. `terraform apply` to apply the changes to your Juju deployment.



## Upgrade `terraform-provider-juju`

To upgrade `terraform-provider-juju`, in your Terraform plan update the version constraint, then run `terraform init` with the `--upgrade` flag.

> See more: Terraform [Version constraints](https://developer.hashicorp.com/terraform/language/providers/requirements#version-constraints), [`terraform init --upgrade`](https://developer.hashicorp.com/terraform/cli/commands/init#upgrade-1)

> <small> **Contributors:** @cderici, @hmlanigan, @simonrichardson, @timclicks, @tmihoc</small>
