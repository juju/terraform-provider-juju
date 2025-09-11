(manage-the-terraform-provider-for-juju)=
# Manage the Terraform Provider for Juju

```{toctree}
:maxdepth: 2
:hidden:

manage-provider/upgrade-provider-to-v1.md
```

(install-the-terraform-provider-for-juju)=
## Install the Terraform Provider for Juju

To install the Terraform Provider for Juju on Linux, macOS, or Windows, you need to install the `terraform` CLI.

> See more: [Hashicorp | Install Terraform](https://developer.hashicorp.com/terraform/install)

For example, on a Linux that supports snaps:

```text
sudo snap install terraform --classic
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
      version = "~> 0.19.0"
      source  = "juju/juju"
    }
  }
}
```

#### b. Configure the `juju` provider to use an existing Juju or JIMM controller

In your Terraform plan, configure the `provider` with the details of your existing, externally managed Juju or JIMM controller.

> See more: {ref}`reference-an-externally-managed-controller`

#### c. Build your deployment

> See more: [How-to guides](../howto/index)


### 2. Apply your Terraform plan

In a terminal, in your project directory, run:

a. (just the first time) `terraform init` to initialise your project;

b. `terraform plan` to stage the changes; and

c. `terraform apply` to apply the changes to your Juju deployment.


## Upgrade the Terraform Provider for Juju

To upgrade the Terraform Provider for Juju, in your Terraform plan update the version constraint, then run `terraform init` with the `--upgrade` flag.

> See more: Terraform [Version constraints](https://developer.hashicorp.com/terraform/language/expressions/version-constraints), [`terraform init --upgrade`](https://developer.hashicorp.com/terraform/cli/commands/init#upgrade-1)

If there are breaking changes between versions, also update your Terraform plans to match the new version.
See below for a guide on upgrading between major versions.

### Upgrade from `v0.x` to `v1.0.0`

See {ref}`upgrade-to-terraform-provider-juju-v-1`.
