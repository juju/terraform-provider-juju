(manage-the-terraform-provider-for-juju)=
# Manage the Terraform Provider for Juju

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

- `ca_certificate` (String) If the controller was deployed with a self-signed certificate: This is the certificate to use for identification. This can also be set by the `JUJU_CA_CERT` environment variable
- `client_id` (String) If using JAAS: This is the client ID (OAuth2.0, created by the external identity provider) to be used. This can also be set by the `JUJU_CLIENT_ID` environment variable
- `client_secret` (String, Sensitive) If using JAAS: This is the client secret (OAuth2.0, created by the external identity provider) to be used. This can also be set by the `JUJU_CLIENT_SECRET` environment variable
- `controller_addresses` (String) This is the controller addresses to connect to, defaults to localhost:17070, multiple addresses can be provided in this format: <host>:<port>,<host>:<port>,.... This can also be set by the `JUJU_CONTROLLER_ADDRESSES` environment variable.
- `password` (String, Sensitive) This is the password of the username to be used. This can also be set by the `JUJU_PASSWORD` environment variable
- `username` (String) This is the username registered with the controller to be used. This can also be set by the `JUJU_USERNAME` environment variable

> See more: [`juju` provider](../reference/index)

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

> See more: [`juju` provider](../reference/index)


##### Using the `juju` CLI

```{important}
This method is only supported for regular Juju controllers.
```

In your Terraform plan, leave the `provider` specification empty:

```terraform
provider "juju" {}
```

Then, in a terminal, use the `juju` client to switch to the desired controller: `juju switch <controller>`. Your Terraform plan will be interpreted relative to that controller.

> See more: [`juju` provider](../reference/index)


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
