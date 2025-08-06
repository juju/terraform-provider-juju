(manage-controllers)=
# Manage controllers

> See also: {external+juju:ref}`Juju | Controller <controller>`

The Terraform Provider for Juju does not support controller bootstrap. However, you can make your Terraform plan relative to a specific externally managed Juju or JIMM controller, and you can also use Terraform to control access to a JIMM-managed Juju controller.

(reference-an-externally-managed-controller)=
## Reference an externally managed controller

To reference a controller that you've created outside of Terraform (because Terraform does not support controller bootstrap), in your `provider` definition add the controller details. You can do this in one of 3 ways: using static credentials, using environment variables, or using the `juju` client. Note: The last method is only supported for regular (non-JIMM) Juju controllers.

```{tip}
For all methods: To view your controller’s details, run `juju show-controller --show-password`.
```

### Using static credentials

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
# For a regular Juju controller, provide the username and password:
export JUJU_USERNAME="<username>"
export JUJU_PASSWORD="<password>"
# For a JIMM controller, provide the client ID and client secret:
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

(add-a-cloud-to-a-controller)=
## Add a cloud to a controller

While your controller is implicitly connected to the cloud that it has been bootstrapped on, and can implicitly use that cloud to provision resources, as is generally the case in Juju, you can also give it access to further clouds. The Terraform Provider for Juju currently supports this only for Kubernetes clouds.

> See more: {ref}`add-a-kubernetes-cloud`

(manage-access-to-a-controller)=
## Manage access to a controller

```{note}
At present the Terraform Provider for Juju supports controller access management only for Juju controllers added to JIMM.
```

When using Juju with JAAS, to grant access to a Juju controller added to JIMM, in your Terraform plan add a resource type `juju_jaas_access_controller`. Access can be granted to one or more users, service accounts, roles, and/or groups. You must specify the model UUID, the JAAS controller access level, and the desired list of users, service accounts, roles, and/or groups. For example:

```terraform
resource "juju_jaas_access_controller" "development" {
  access           = "administrator"
  users            = ["foo@domain.com"]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
  roles            = [juju_jaas_role.development.uuid]
  groups           = [juju_jaas_group.development.uuid]
}
```

> See more: [`juju_jaas_access_controller`](../reference/terraform-provider/resources/jaas_access_controller), {external+jaas:ref}`JAAS | Controller access levels <list-of-controller-permissions>`
