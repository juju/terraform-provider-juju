---
myst:
  html_meta:
    description: "Learn how to reference and connect to Juju and JIMM controllers using static credentials, environment variables, or the Juju CLI."
---

(manage-controllers)=
# Manage controllers

> See also: {external+juju:ref}`Juju | Controller <controller>`

(bootstrap-a-controller)=
## Bootstrap a controller

To bootstrap a new Juju controller use the `juju_controller` resource.

### Bootstrap to LXD (localhost)

This example bootstraps a controller onto the local LXD cloud using certificate authentication.

**1. Configure the provider for controller mode.**

Bootstrapping is a unique situation as there is no controller to connect to yet so our `provider` block will be mostly empty.

Set `controller_mode = true` in the provider to enable bootstrapping.\
No resources besides controllers can be created when this flag is set.

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

**2. Obtain your LXD credential values (including secrets):**

```bash
juju show-credentials --client localhost localhost --show-secrets
```

From the output, you will need the values `client-cert`, `client-key`, and `server-cert`.
Keep them out of version control (for example, pass them via `TF_VAR_...` environment variables, a secrets manager, or a `.tfvars` file you do not commit).

**3. Declare the controller:**

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

**4. Change config post-bootstrap:**

After bootstrap, the controller config and controller-model config can be changed.

Note the following behaviors:
1. If you remove a key from `controller_config`, it will not be unset on the controller; it is left unchanged.
2. Attempting to change a config value that Juju doesn't support changing after bootstrap will result in an error. You must destroy and recreate the controller to change these values.
3. Boolean values must be specified as either "true" or "false".

```{tip}
Many `juju_controller` fields correspond to the same flags used by the `juju bootstrap` CLI. When in doubt, `juju bootstrap --help` and the Juju docs are a good way to discover valid keys and values.
```

```terraform
resource "juju_controller" "this" {
  # additional fields ommitted

  controller_config = {
    "audit-log-max-backups"     = "10"
  }

  controller_model_config = {
    "juju-http-proxy"              = "http://my-proxy.internal"
    "update-status-hook-interval"  = "1m"
  }
}
```

> See more: [`juju_controller` (resource)](../reference/terraform-provider/resources/controller)


(add-a-cloud-to-a-controller)=
## Add a cloud to a controller

> See more: {ref}`add-a-machine-cloud`
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
