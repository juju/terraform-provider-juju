(manage-controllers)=
# Manage controllers

> See also: {external+juju:ref}`Juju | Controller <controller>`

(bootstrap-a-controller)=
## Bootstrap a controller

Use the `juju_controller` resource to bootstrap a brand new Juju controller as part of a Terraform plan.

```{important}
Bootstrapping is a special case: there is no controller to connect to yet.
Set `controller_mode = true` in the provider so Terraform can create a controller.
No other resources can be created besides controllers when this flag is set.
```

### Bootstrap to LXD (localhost)

This example bootstraps a controller onto the local LXD cloud using certificate authentication.

1. Configure the provider for controller mode. 

```terraform
terraform {
  required_providers {
    juju = {
      source  = "juju/juju"
      version = "~> 1.0.0"
    }
  }
}

provider "juju" {
  controller_mode = true
}
```

2. Obtain your LXD credential values (including secrets):

```bash
juju show-credentials --client localhost localhost --show-secrets
```

From the output, you will need the values `client-cert`, `client-key`, and `server-cert`.
Keep them out of version control (for example, pass them via `TF_VAR_...` environment variables, a secrets manager, or a `.tfvars` file you do not commit).

3. Declare the controller:

Here we use the `localhost` cloud which is already known to the Juju CLI. Private clouds can be specified by filling the remainder of the fields in the `cloud` object.

```terraform
resource "juju_controller" "this" {
  name = "test-controller"

  cloud = {
    name       = "localhost"
    type       = "lxd"
    auth_types = ["certificate"]
  }

  cloud_credential = {
    name      = "localhost"
    auth_type = "certificate"
    attributes = {
      "client-cert" = var.lxd_client_cert
      "client-key"  = var.lxd_client_key
      "server-cert" = var.lxd_server_cert
    }
  }

  # Settings here map to flags/config used by `juju controller-config`.
  controller_config = {
    "audit-log-max-backups"     = "10"
    "query-tracing-enabled"     = "true"
  }

  # Settings here map to flags/config used by `juju model-config`.
  controller_model_config = {
    "juju-http-proxy" = "http://my-proxy.internal"
  }

  # Optional: use a Juju binary from a specific location.
  # The default is /usr/bin/juju.
  juju_binary = "/snap/juju/current/bin/juju"
}
```

```{important}
If you have installed Juju as a snap, use the path `/snap/juju/current/bin/juju` to avoid snap confinement issues.
```

After `terraform apply`, the resource exposes useful read-only attributes such as the controller `api_addresses`, `ca_cert`, `username`, and `password`.

### Bootstrap configuration

Many `juju_controller` arguments correspond to the same concepts used by the `juju bootstrap` CLI. When in doubt, `juju bootstrap --help` and the Juju docs are a good way to discover valid keys and values.

The following behaviours are important caveats in how the `juju_controller` resource handles config:
1. If you remove a key from `controller_config`, it will not be unset on the controller; it is left unchanged.
2. Attempting to change a config value that cannot be changed after bootstrap will result in an error. You must destroy and recreate the controller to change these values.
3. Values besides strings and integers must follow a specific format, currently this only applies to boolean values which must be "true"/"false".

> See more: [`juju_controller` (resource)](../reference/terraform-provider/resources/controller)


(add-a-cloud-to-a-controller)=
## Add a cloud to a controller

While your controller is implicitly connected to the cloud that it has been bootstrapped on, and can implicitly use that cloud to provision resources, as is generally the case in Juju, you can also give it access to further clouds. The Terraform Provider for Juju currently supports this only for Kubernetes clouds.

> See more: {ref}`add-a-kubernetes-cloud`

(add-a-credential-to-a-controller)=
## Add a credential to a controller

By virtue of being bootstrapped into a cloud, your controller already has a credential for that cloud. However, if you want to use a different credential, or if you're adding a further cloud to the controller and would like to also add a credential for that cloud, you will need to add those credentials to the controller too. You can do that in the usual way by creating a resource of the `juju_credential` type.

> See more: {ref}`add-a-credential`

(manage-access-to-a-controller)=
## Manage access to a controller

```{note}
At present the Terraform Provider for Juju supports controller access management only for Juju controllers added to JAAS.
```

When using Juju with JAAS, to grant access to a Juju controller added to JAAS, in your Terraform plan add a resource type `juju_jaas_access_controller`. Access can be granted to one or more users, service accounts, roles, and/or groups. You must specify the model UUID, the JAAS controller access level, and the desired list of users, service accounts, roles, and/or groups. For example:

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
