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

To bootstrap a new Juju controller, configure the Terraform provider in controller mode, obtain cloud credentials for your target cloud, and define a `juju_controller` resource specifying the controller name, cloud configuration, and credentials. You can also configure the controller with various settings during bootstrap or update them later.

````{dropdown} Preview an example workflow: Bootstrap to LXD
This example bootstraps a controller onto the local LXD cloud using certificate authentication.

```{code-block} terraform
:caption: `main.tf`

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

locals {
  lxd_creds = yamldecode(file("~/lxd-credentials.yaml"))
}

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
      "client-cert" = local.lxd_creds.client-cert
      "client-key"  = local.lxd_creds.client-key
      "server-cert" = local.lxd_creds.server-cert
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

```{code-block} yaml
:caption: `~/lxd-credentials.yaml`

client-cert: |
  -----BEGIN CERTIFICATE-----
  <your client certificate>
  -----END CERTIFICATE-----
client-key: |
  -----BEGIN PRIVATE KEY-----
  <your client key>
  -----END PRIVATE KEY-----
server-cert: |
  -----BEGIN CERTIFICATE-----
  <your server certificate>
  -----END CERTIFICATE-----
```

Obtain credential values by running:

```bash
juju show-credentials --client localhost localhost --show-secrets
```

From the output, extract the certificate values and save them to `~/lxd-credentials.yaml`.

```{important}
If you have installed Juju as a snap, use the path `/snap/juju/current/bin/juju` to avoid snap confinement issues.
```
````

In more detail:

**1. Set up the provider in controller mode**

Configure the provider with `controller_mode = true`. This enables bootstrapping and restricts resource creation to controllers only.

> See more: {ref}`Set up the provider in controller mode (bootstrapping) <set-up-the-terraform-provider-for-juju>`

**2. Obtain cloud credentials**

Gather the necessary cloud credentials for your target cloud (e.g., LXD, AWS, Kubernetes). These typically include authentication certificates, keys, or tokens.

**3. Define the controller resource**

Create a `juju_controller` resource with your controller name, cloud configuration, and credentials. You can also include `controller_config` and `controller_model_config` to configure the controller during bootstrap.

After `terraform apply`, the resource exposes useful read-only attributes such as the controller `api_addresses`, `ca_cert`, `username`, and `password`.

> See more: [`juju_controller` (resource)](../reference/terraform-provider/resources/controller), {ref}`configure-a-controller`


(configure-a-controller)=
## Configure a controller

> See also: {external+juju:ref}`Juju | Configuration <configuration>`, {external+juju:ref}`Juju | List of controller configuration keys <list-of-controller-configuration-keys>`

A Juju controller can be configured with various settings that control its behavior. There are two types of configuration:

- **Controller configuration** (`controller_config`) - Settings specific to the controller itself
- **Controller model configuration** (`controller_model_config`) - Default model settings for the controller model

You can configure these settings either during bootstrap or after the controller is created.

### Configure during bootstrap

To configure a controller during bootstrap, in your `juju_controller` resource specify the `controller_config` and/or `controller_model_config` attributes. For example:

```terraform
resource "juju_controller" "this" {
  name        = "configured-controller"
  juju_binary = "/snap/juju/current/bin/juju"

  cloud = {
    # cloud configuration...
  }

  cloud_credential = {
    # credential configuration...
  }

  # Controller-specific configuration
  controller_config = {
    "audit-log-max-backups"  = "10"
    "query-tracing-enabled"  = "true"
  }

  # Controller model configuration
  controller_model_config = {
    "juju-http-proxy"   = "http://my-proxy.internal"
    "disable-telemetry" = "true"
  }
}
```

### Configure post-bootstrap

To update controller configuration after bootstrap, modify the `controller_config` or `controller_model_config` attributes in your Terraform configuration and run `terraform apply`:

```terraform
resource "juju_controller" "this" {
  # ... existing configuration ...

  controller_config = {
    "audit-log-max-backups"  = "15"      # Updated from 10
    "query-tracing-enabled"  = "true"
    "audit-log-capture-args" = "true"    # Newly added
  }
}
```

```{important}
**Configuration update behaviors:**
1. **Removing a key** from `controller_config` does not unset it on the controller - it remains at its previous value
2. **Some settings cannot be changed** after bootstrap. Attempting to change them will result in an error, requiring you to destroy and recreate the controller
3. **Boolean values** must be specified as strings: `"true"` or `"false"`, not bare boolean values

To restore a setting to its default value, you must explicitly set it to the default value rather than removing it from the configuration.

To discover valid configuration keys and values, use `juju bootstrap --help` or consult the Juju documentation. Many `juju_controller` resource attributes correspond directly to the flags and config options used by the `juju bootstrap` CLI.
```

### View controller configuration

To view the current controller configuration managed by Terraform:

```bash
# View full controller state including configuration
terraform state show juju_controller.this

# Or view specific attributes
terraform state show juju_controller.this | grep -A 10 controller_config
```

The Terraform state reflects the configuration values you've set. To see all configuration values including defaults set by Juju, use the `juju` CLI:

```bash
juju controller-config -c <controller-name>
```

> See more: {external+juju:ref}`juju controller-config <command-juju-controller-config>`


(enable-controller-high-availability)=
## Enable controller high availability

```{note}
Enabling HA relies on Terraform actions, which require **Terraform 1.14** or later. For more information, see [Terraform actions](https://developer.hashicorp.com/terraform/language/v1.14.x/invoke-actions).
```

High availability (HA) for a Juju controller ensures that multiple controller units are running so the controller remains available if individual units fail. You can enable HA either during bootstrap or post-bootstrap, and in the latter case you can scale out as well as in.

### Enable controller high availability during bootstrap

To enable HA during bootstrap, in your `juju_controller` resource, in the `lifecycle` block, define the `action_trigger` field.

```terraform
resource "juju_controller" "this" {
  name        = "my-controller"
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

  lifecycle {
    ignore_changes = [
      cloud_credential.attributes["client-cert"],
      cloud_credential.attributes["client-key"],
    ]
    action_trigger {
      events  = [after_create]
      actions = [action.juju_enable_ha.this]
    }
  }
}

action "juju_enable_ha" "this" {
  config {
    api_addresses = juju_controller.this.api_addresses
    ca_cert       = juju_controller.this.ca_cert
    username      = juju_controller.this.username
    password      = juju_controller.this.password
    units         = 3
  }
}
```

### Enable controller high availability post bootstrap

To enable controller HA post bootstrap, define a Terraform juju_enable_ha action block:

```terraform
action "juju_enable_ha" "this" {
  config {
    api_addresses = juju_controller.this.api_addresses
    ca_cert       = juju_controller.this.ca_cert
    username      = juju_controller.this.username
    password      = juju_controller.this.password
    units         = 5
  }
}
```

Then run:

```bash
terraform apply -invoke=action.juju_enable_ha.this
```

Terraform will execute the `juju_enable_ha` action and ensure the controller has the requested number of units.

### Update the number of units

```{note}
  As with the `juju` CLI, constraints set while scaling in post bootstrap always apply only to the new units being created.
```

To scale out the number of units via the terraform *enable_ha* action. The number of units must always be an odd number.

To scale an HA controller in, remove its backing machines manually  via the `juju` CLI [`juju remove-machine`](https://documentation.ubuntu.com/terraform-provider-juju/latest/howto/manage-machines/#remove-a-machine).

```{note}
  While it _is_ possible to control the number of units or remove machines directly through Terraform, that is currently supported only for regular applications.
```

(import-an-existing-controller)=
## Import an existing controller

To import an existing controller into Terraform, gather its connection details from `juju show-controller`, define a matching `juju_controller` resource with cloud and credential configuration, create an import block with the connection information, and add lifecycle rules to ignore attributes that differ between your plan and the controller's actual state.

```{note}
This operation imports the controller as a **resource** that Terraform will manage. Controllers cannot be referenced as data sources (read-only). Once imported, Terraform will track the controller's state and can make changes to its configuration.
```

````{dropdown} Preview an example workflow: Import LXD controller
This example shows a complete workflow for importing an LXD controller.

```{code-block} terraform
:caption: `main.tf`

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
      cloud_credential.attributes["client-key"],
    ]
  }
}

import {
  to = juju_controller.imported
  identity = {
    name            = "my-lxd-controller"
    api_addresses   = ["<ip>:17070"]
    username        = "admin"
    password        = "<password>"
    ca_cert         = <<-EOT
      -----BEGIN CERTIFICATE-----
      -----END CERTIFICATE-----
    EOT
    controller_uuid = "<controller-uuid>"
    credential_name = "<credential-name>"
  }
}
```

```{note}
The `cloud_credential.attributes["client-cert"]` and `cloud_credential.attributes["client-key"]` are not required to bootstrap an LXD controller, but they are populated in the state during import because they are fetched from the controller. The same applies to `cloud.endpoint` and `cloud.region`, which may be set by Juju during bootstrap even if not explicitly specified.
```
````

````{dropdown} Preview an example workflow: Import MicroK8s controller
This example shows a complete workflow for importing a MicroK8s controller.

```{code-block} terraform
:caption: `main.tf`

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

import {
  to = juju_controller.imported
  identity = {
    name            = "my-k8s-controller"
    api_addresses   = ["<ip>:17070"]
    username        = "admin"
    password        = "<password>"
    ca_cert         = <<-EOT
      -----BEGIN CERTIFICATE-----
      -----END CERTIFICATE-----
    EOT
    controller_uuid = "<controller-uuid>"
    credential_name = "<credential-name>"
  }
}
```

```{note}
The `cloud.region` is not required during bootstrap but may be set by Juju and needs to be ignored. The `cloud.host_cloud_region` cannot be fetched from the controller after bootstrap, so it must be ignored to prevent Terraform from attempting to replace the controller.
```
````

In more detail:

**1. Get controller connection information.**

To import a controller, you need its connection details. You can obtain these by running:

```bash
juju show-controller --show-password
```

**2. Create an import block.**

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
    controller_uuid = "<controller-uuid>"
    credential_name = "<credential-name>"
  }
}
```

**3. Define a `juju_controller` resource.**

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

Then run `terraform plan`. Terraform will detect the import block and import the controller during the next `terraform apply`.


> See more: [`juju_controller` (resource)](../reference/terraform-provider/resources/controller)

**4. Verify the imported controller.**

After importing:

a. Run `terraform plan` to see which attributes Terraform cannot determine or that differ from your configuration. These differences are expected after an import.

b. Add any fields showing unexpected changes to the `lifecycle.ignore_changes` block. Common fields to ignore include:
   - Credential attributes that may differ between your plan and the ones fetched from the controller
   - Cloud region and endpoint fields, which may be set by Juju during bootstrap even if not explicitly specified
   - Bootstrap-time configuration that cannot be changed and can't be fetched from the controller

c. Run `terraform plan` again. You should see either no changes or only expected configuration updates.

```{tip}
If you see `controller_config` or `controller_model_config` showing changes to set default values, you can either apply them (they will update the controller configuration which is idempotent) or add these blocks to `ignore_changes` to prevent the updates.
```

(add-a-cloud-to-a-controller)=
## Add a cloud to a controller

> See more: {ref}`add-a-machine-cloud`, {ref}`add-a-kubernetes-cloud`

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

(remove-a-controller)=
## Remove a controller

> See also: {external+juju:ref}`Juju | Removing things <removing-things>`

To remove a controller, remove its resource definition from your Terraform plan.

> See more: [`juju_controller` (resource)](../reference/terraform-provider/resources/controller)
