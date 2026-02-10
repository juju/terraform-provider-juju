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

  juju_binary = "/snap/juju/current/bin/juju"
  
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


(import-an-existing-controller)=
## Import an existing controller

If you have a controller that was created outside of Terraform (for example, via `juju bootstrap`), you can import it into your Terraform state.

**Import syntax:**

To import an existing controller, you must use the identity-based import format. This format requires you to specify the controller's connection details in an `import` block.

```{important}
Importing controllers by ID is not supported. You must use the identity schema in your import block.
```

**Getting controller connection information:**

To import a controller, you need its connection details. You can obtain these by running:

```bash
juju show-controller --show-password
```

From the output, you will need:
- Controller name
- API addresses
- CA certificate
- Admin username (typically `admin`)
- Admin password
- Controller UUID
- Credential name

**Import block structure:**

Create an `import` block with the identity schema containing the controller's connection information:

```terraform
import {
  to = juju_controller.imported
  identity = {
    name            = "my-existing-controller"
    api_addresses   = ["<ip>:17070"]
    username        = "admin"
    password        = "<password>"
    ca_cert         = <<-EOT
      -----BEGIN CERTIFICATE-----
      -----END CERTIFICATE-----
    EOT
    controller_uuid = "<controller-uudi>"
    credential_name = "<credential-name>"
  }
}
```

**Resource configuration:**

You also need to define the corresponding `juju_controller` resource with the cloud and credential information:

```terraform
resource "juju_controller" "imported" {
  name = "my-existing-controller"

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
}
```

Then run:

```bash
terraform plan
```

Terraform will detect the import block and import the controller during the next `terraform apply`.

(import-example-lxd)=
### Import example: LXD controller

```terraform
provider "juju" {
  controller_mode = true
}

resource "juju_controller" "imported" {
  name = "my-lxd-controller"

  juju_binary = "/snap/juju/current/bin/juju"

  cloud = {
    name       = "localhost"
    type       = "lxd"
    auth_types = ["certificate"]
  }

  cloud_credential = {
    name      = "localhost"
    auth_type = "certificate"
    attributes = {
      <attrs>
    }
  }

  lifecycle {
    ignore_changes = [
      cloud.endpoint,
      cloud.region,
      cloud_credential.attributes["client-cert"],
      cloud_credential.attributes["client-key"]
    ]
  }
}
```

```{note}
The `cloud_credential.attributes["client-cert"]` and `cloud_credential.attributes["client-key"]` are not required to bootstrap an LXD controller, but they are populated in the state during import because they are fetched from the controller. The same applies to `cloud.endpoint` and `cloud.region`, which may be set by Juju during bootstrap even if not explicitly specified.
```

(import-example-microk8s)=
### Import example: MicroK8s controller

```terraform
provider "juju" {
  controller_mode = true
}

resource "juju_controller" "imported" {
  name = "my-k8s-controller"

  juju_binary = "/snap/juju/current/bin/juju"

  cloud = {
    name                = "test-k8s"
    type                = "kubernetes"
    auth_types          = ["clientcertificate"]
    endpoint            = var.k8s_endpoint
    ca_certificates     = [var.k8s_ca_cert]
    host_cloud_region   = "localhost"
  }

  cloud_credential = {
    name      = "test-credential"
    auth_type = "clientcertificate"
    attributes = {
      "ClientCertificateData" = var.k8s_client_cert
      "ClientKeyData"         = var.k8s_client_key
    }
  }

  lifecycle {
    ignore_changes = [
      cloud.region,
      cloud.host_cloud_region
    ]
  }
}
```

```{note}
The `cloud.region` is not required during bootstrap but may be set by Juju and needs to be ignored. The `cloud.host_cloud_region` cannot be fetched from the controller after bootstrap, so it must be ignored to prevent Terraform from attempting to replace the controller.
```

(import-post-import-workflow)=
### Post-import workflow

After importing a controller:

**1. Review the plan:**

Run `terraform plan` to see which attributes Terraform cannot determine or that differ from your configuration. These differences are expected after an import.

```bash
terraform plan
```

**2. Add necessary ignore_changes:**

Based on the plan output, add any fields showing unexpected changes to the `lifecycle.ignore_changes` block that would require a replace of the controller resource.  
Common fields to ignore include:

- Credential attributes that may differ between your plan and the ones fetched from the controller. 
- Cloud region and endpoint fields, which can be default when a controller is bootstrap, but it's returned when it's set in the state when it's fetched from the controller.
- Bootstrap-time configuration that cannot be changed, and can't be fetched from the controller.

**3. Verify the configuration:**

After adding the appropriate `lifecycle.ignore_changes` directives, run `terraform plan` again. You should see either no changes or only expected configuration updates.

```{tip}
If you see `controller_config` or `controller_model_config` showing changes to set default values, you can either apply them (they will update the controller configuration which is idempotent) or add these blocks to `ignore_changes` to prevent the updates.
```


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
