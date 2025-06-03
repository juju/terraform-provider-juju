(manage-the-terraform-provider-for-juju)=
# Manage the Terraform Provider for Juju

## Install the Terraform Provider for Juju

To install the Terraform Provider for Juju on Linux, macOS, or Windows, you need to install the `terraform` CLI.

> See more: [Hashicorp | Install Terraform](https://developer.hashicorp.com/terraform/install)

For example, on a Linux that supports snaps:

```text
sudo snap install terraform
```

(use-the-terraform-provider-for-juju)=
## Use the Terraform Provider for Juju

To use the Terraform Provider for Juju, create a Terraform plan specifying the `juju` provider, an existing Juju or JIMM controller, and resources or data sources for whatever Juju entities you want to deploy, then apply your plan in the usual Terraform way.

### 1. Build your Terraform plan

#### a. Configure Terraform to use the `juju` provider

In your Terraform plan, add:

```terraform
terraform {
  required_providers {
    juju = {
      version = "~> 0.13.0"
      source  = "juju/juju"
    }
  }
}
```

#### b. Configure the `juju` provider to use an existing Juju or JIMM controller

There are 3 ways you can do this: using static credentials, using environment variables, or using the `juju` client. The last method is only supported for regular Juju controllers.

```{tip}
For all methods: To view your controller’s details, run `juju show-controller --show-password`.
```

##### Using static credentials

In your Terraform plan add:

```terraform
provider "juju" {
  controller_addresses = "<controller addresses>"
  # For a controller deployed with a self-signed certificate:
  ca_certificate = file("<path to certificate file>")
  # For a regular Juju controller, provide the username and password:
  username = "<username>"
  password = "<password>"
  # For a JIMM controller, provide the client ID and client secret:
  client_id     = "<clientID>"
  client_secret = "<clientSecret>"
}
```

> See more: [Terraform | `juju` provider](https://registry.terraform.io/providers/juju/juju/latest/docs)

##### Using environment variables

In your Terraform plan, leave the `provider` specification empty:

```terraform
provider "juju" {}
```

Then, in a terminal, export the controller environment variables with your controller's values. For example:

```bash
export JUJU_CONTROLLER_ADDRESSES="<controller addresses>"
# For a controller deployed with a self-signed certificate:
export JUJU_CA_CERT=file("<path to certificate file>")
# For a regular Juju controller, provide the username and password:
export JUJU_USERNAME="<username>"
export JUJU_PASSWORD="<password>"
# For a JIMM controller, provide the client ID and client secret:
export JUJU_CLIENT_ID="<client ID>"
export JUJU_CLIENT_SECRET="<client secret>"
```

> See more: [Terraform | `juju` provider](https://registry.terraform.io/providers/juju/juju/latest/docs)


##### Using the `juju` CLI

```{important}
This method is only supported for regular Juju controllers.
```

In your Terraform plan, leave the `provider` specification empty:

```terraform
provider "juju" {}
```

Then, in a terminal, use the `juju` client to switch to the desired controller: `juju switch <controller>`. Your Terraformplan will be interpreted relative to that controller.

> See more: [Terraform | `juju` provider](https://registry.terraform.io/providers/juju/juju/latest/docs)


#### c. Build your deployment

> See more: [How-to guides](../howto/index)


### 2. Apply your Terraform plan

In a terminal, in your project directory, run:

a. (just the first time) `terraform init` to initialise your project;

b. `terraform plan` to stage the changes; and

c. `terraform apply` to apply the changes to your Juju deployment.


## Upgrade the Terraform Provider for Juju

To upgrade the Terraform Provider for Juju, in your Terraform plan update the version constraint, then run `terraform init` with the `--upgrade` flag.

> See more: Terraform [Version constraints](https://developer.hashicorp.com/terraform/language/providers/requirements#version-constraints), [`terraform init --upgrade`](https://developer.hashicorp.com/terraform/cli/commands/init#upgrade-1)
