---
myst:
  html_meta:
    description: "How to bootstrap and use a controller in Terraform."
---

(use-the-bootstrapped-controller)=
# Use a bootstrapped controller

To bootstrap and use a controller you need two plans:

1. A bootstrap plan that creates the controller and writes connection info to a JSON file.
2. A second plan that reads that JSON file to configure the provider and deploy a test model.

Start with the bootstrap guidance in {ref}`bootstrap-a-controller` from {ref}`manage-controllers`.

## Plan 1: Bootstrap and write connection info

In the bootstrap plan, add a `local_file` resource to write a JSON file with the controller connection details. This uses the outputs from `juju_controller`.

```terraform
terraform {
  required_providers {
    juju = {
      source = "juju/juju"
    }
    local = {
      source = "hashicorp/local"
    }
  }
}

provider "juju" {
  controller_mode = true
}

locals {
  lxd_creds = yamldecode(file("~/lxd-credentials.yaml"))
}

resource "juju_controller" "controller" {
  name        = "controller"
  juju_binary = "/snap/juju/current/bin/juju"

  cloud = {
    name       = "localhost"
    auth_types = ["certificate"]
    type       = "lxd"
    endpoint   = local.lxd_creds.endpoint

    region = {
      name = "localhost"
    }
  }

  cloud_credential = {
    name      = "localhost"
    auth_type = "certificate"

    attributes = {
      server-cert = local.lxd_creds.server-cert
    }
  }

  lifecycle {
    ignore_changes = [
      cloud_credential.attributes["client-cert"],
      cloud_credential.attributes["client-key"],
    ]
  }
}

resource "local_file" "conn_info_json" {
  filename = "${path.module}/conn_info.json"
  content = jsonencode({
    username  = juju_controller.controller.username
    password  = juju_controller.controller.password
    addresses = juju_controller.controller.api_addresses
    ca_cert   = juju_controller.controller.ca_cert
  })
}
```

## Plan 2: Read JSON and deploy a test model

In the second plan, read the JSON file to configure the provider, then add a model resource (for example, `test-model`).

```terraform
terraform {
  required_providers {
    juju = {
      source = "juju/juju"
    }
  }
}

locals {
  conn_info = jsondecode(file("${path.module}/../bootstrap/conn_info.json"))
}

provider "juju" {
  controller_addresses = join(",", local.conn_info.addresses)
  username             = local.conn_info.username
  password             = local.conn_info.password
  ca_certificate       = local.conn_info.ca_cert
}

resource "juju_model" "test" {
  name = "test-model"
}
```

```{note}
Keep conn_info.json private because it contains credentials.
```

## Why this must be two plans

Terraform does not support configuring a provider block from resource outputs in the same plan. While it may appear to work in some cases, it is not a supported Terraform feature and may break at any time. For that reason, keep bootstrapping (controller creation) and controller use (provider configuration for resources) in separate plans.

## Controller replacement and state cleanup

If the controller created by the bootstrap plan is destroyed or replaced, the second plan’s state will still reference resources tied to the old controller. In that case, you must manually remove the second plan’s state (or the affected resources) before reapplying. As a rule of thumb, destroy the second plan first, then destroy or replace the controller in the bootstrap plan.
