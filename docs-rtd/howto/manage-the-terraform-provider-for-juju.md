---
myst:
  html_meta:
    description: "Learn how to install, configure, and upgrade the Terraform Provider for Juju with detailed setup instructions and version migration guides."
---

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

(setup-provider)=
## Set up the Terraform Provider for Juju

The provider supports two modes: **controller mode** (for bootstrapping) and **regular mode** (for using existing controllers).

### For controller mode (bootstrapping)

To bootstrap a new controller, in your Terraform plan (e.g., `main.tf`) define the provider with `controller_mode = true`:

```terraform
provider "juju" {
  controller_mode = true
}
```

In the same plan, define a `juju_controller` resource to bootstrap your controller. No other resources can be created when this flag is set.

> See more: {ref}`bootstrap-a-controller`

### For regular mode (using existing controllers)

To connect to an existing controller, choose one of three authentication methods:

1. Static credentials in your Terraform plan;
2. Environment variables;
3. The `juju` CLI (not supported for JAAS controllers).

For Juju controllers, provide username and password. For JAAS controllers, provide client ID and client secret from your external identity provider.

```{tip}
To view your controller's details, run `juju show-controller --show-password`. No password will be shown for JAAS controllers.
```

#### Using static credentials

In your Terraform plan add:

```terraform
provider "juju" {
  controller_addresses = "<controller addresses>"
  # For a controller deployed with a self-signed certificate:
  ca_certificate = file("<path to certificate file>")
  # For a regular Juju controller, provide the username and password for a user:
  username = "<username>"
  password = "<password>"
  # For a JAAS controller, provide the client ID and client secret for a service account
  # (OAuth 2.0 credentials from your external identity provider):
  client_id     = "<clientID>"
  client_secret = "<clientSecret>"
}
```

All parameters can alternatively be set via environment variables:

- `ca_certificate` → `JUJU_CA_CERT`
- `client_id` → `JUJU_CLIENT_ID`
- `client_secret` → `JUJU_CLIENT_SECRET`
- `controller_addresses` → `JUJU_CONTROLLER_ADDRESSES` (defaults to localhost:17070; supports multiple: `<host>:<port>,<host>:<port>,...`)
- `password` → `JUJU_PASSWORD`
- `username` → `JUJU_USERNAME`

Keep sensitive values out of version control (use `TF_VAR_...` environment variables, a secrets manager, or an uncommitted `.tfvars` file).

> See more: [`juju` provider](../reference/index)

#### Using environment variables

In your Terraform plan, define an empty provider:

```terraform
provider "juju" {}
```

Then, in a terminal, export the controller environment variables with your controller's values. For example:

```bash
export JUJU_CONTROLLER_ADDRESSES="<controller addresses>"
# For a controller deployed with a self-signed certificate:
export JUJU_CA_CERT=file("<path to certificate file>")
# For a regular Juju controller, provide the username and password for a user:
export JUJU_USERNAME="<username>"
export JUJU_PASSWORD="<password>"
# For a JAAS controller, provide the client ID and client secret for a service account:
export JUJU_CLIENT_ID="<client ID>"
export JUJU_CLIENT_SECRET="<client secret>"
```

> See more: [`juju` provider](../reference/index)

#### Using the `juju` CLI

```{important}
Not supported for JAAS controllers.
```

In your Terraform plan, leave the `provider` specification empty:

```terraform
provider "juju" {}
```

Then, in a terminal, use the `juju` client to switch to the desired controller: `juju switch <controller>`. Your Terraform plan will be interpreted relative to that controller.

> See more: [`juju` provider](../reference/index)


(use-the-terraform-provider-for-juju)=
## Use the Terraform Provider for Juju

To use the Terraform Provider for Juju, create a Terraform plan specifying the `juju` provider, an existing controller, and resources or data sources for whatever Juju entities you want to deploy, then apply your plan in the usual Terraform way.

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

In your Terraform plan, configure the `provider` with the details of your existing, Juju or JIMM controller.

> See more: {ref}`setup-provider`

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
