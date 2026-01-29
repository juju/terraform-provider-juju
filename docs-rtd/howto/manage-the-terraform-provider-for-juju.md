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

To set up the provider, connect it to a Juju controller. You can do this in one of 3 ways:

1. Using static credentials;
2. Using environment variables;
3. Using the `juju` client. 

Use of the `juju` client for configuration is not supported for JAAS controllers. 

Across all the supported methods, for authentication with a Juju controller you must provide the username and password for a user, whereas for authentication with a JAAS controller you must provide the client ID and client secret for a service account (where the service account must be created through the external identity provider connected to the JAAS controller).


```{tip}
To view your controller’s details, run `juju show-controller --show-password`. No password will be shown for JAAS controllers.
```

### Using static credentials

In your Terraform plan add:

```terraform
provider "juju" {
  controller_addresses = "<controller addresses>"
  # For a controller deployed with a self-signed certificate:
  ca_certificate = file("<path to certificate file>")
  # For a regular Juju controller, provide the username and password for a user:
  username = "<username>"
  password = "<password>"
  # For a JAAS controller, provide the client ID and client secret for a service account:
  client_id     = "<clientID>"
  client_secret = "<clientSecret>"
}
```

- `ca_certificate` (String) If the controller was deployed with a self-signed certificate: This is the certificate to use for identification. This can also be set by the `JUJU_CA_CERT` environment variable
- `client_id` (String) If using JAAS: This is the client ID (OAuth2.0, created by the external identity provider) to be used. This can also be set by the `JUJU_CLIENT_ID` environment variable
- `client_secret` (String, Sensitive) If using JAAS: This is the client secret (OAuth2.0, created by the external identity provider) to be used. This can also be set by the `JUJU_CLIENT_SECRET` environment variable
- `controller_addresses` (String) This is the controller addresses to connect to, defaults to localhost:17070, multiple addresses can be provided in this format: `<host>:<port>,<host>:<port>,...` This can also be set by the `JUJU_CONTROLLER_ADDRESSES` environment variable.
- `password` (String, Sensitive) This is the password of the username to be used. This can also be set by the `JUJU_PASSWORD` environment variable
- `username` (String) This is the username registered with the controller to be used. This can also be set by the `JUJU_USERNAME` environment variable

Sensitive values should be kept of version control (for example, pass them via `TF_VAR_...` environment variables, a secrets manager, or a `.tfvars` file you do not commit).

> See more: [`juju` provider](../reference/index)

### Using environment variables

The provider also supports specific environment variables for configuration.\
In your Terraform plan, leave the `provider` specification empty:

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


### Using the `juju` CLI

```{important}
This method is only supported for regular Juju controllers.
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
