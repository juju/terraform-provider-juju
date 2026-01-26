(setup-provider)=
# Setup the provider

(reference-a-controller)=
## Reference a controller

To reference a controller in your `provider` definition, add your controller address(es) and your controller authentication details. You can do this in one of 3 ways:
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

> See more: [`juju` provider](../reference/index)

### Using environment variables

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
