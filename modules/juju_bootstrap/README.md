# juju_bootstrap module

This module bootstraps a Juju controller and optionally enables high availability (HA).

## What it does

- Configures the `juju` provider in `controller_mode`.
- Creates a `juju_controller` resource with configurable cloud, credential, and bootstrap settings.
- Optionally runs `juju enable-ha` using a local-exec provisioner when `controller_num_units > 0`.

## Requirements

- Terraform/OpenTofu 1.12+.
- Juju provider: `juju/juju > 1.3`.
- `juju` CLI available on the machine running apply.

## Usage

### LXD cloud

```hcl
module "juju_bootstrap_example" {
  source = "/path/to/module"

  name = "my-controller"

  cloud = {
    auth_types = ["certificate"]
    name       = "lxd-cloud"
    type       = "lxd"
    endpoint   = "https://10.0.0.1:8443"
    region = {
      name     = "default"
      endpoint = "https://10.0.0.1:8443"
    }
  }

  cloud_credential = {
    auth_type = "interactive"
    name      = "lxd-token"
    attributes = {
      trust-token = trimspace(file("/path/to/token"))
    }
  }

  controller_num_units = 3
}
```

### MAAS cloud

```hcl
module "juju_bootstrap_example" {
  source = "/path/to/module"

  name = "my-controller"

  cloud_details = {
    auth_types = ["oauth1"]
    type     = "maas"
    name     = "maas-cloud"
    endpoint  = "http://10.0.0.1:5240/MAAS/"
    api_key  = trimspace(file("/path/to/maas_api_key"))
  }

  cloud_credential = {
    auth_type = "oauth1"
    name      = "maas-creds"
    attributes = {
      maas-oauth  = trimspace(file("/path/to/maas_api_key"))
      }
    }

  controller_num_units = 3
}
```

### Example of controller_model_config or model_default
```
controller_model_config = {
  default-base = "ubuntu@22.04"
  lxd-snap-channel = "5.0/stable"
  cloudinit-userdata = <<EOT
#cloud-config

ca-certs:
  trusted:
  - |
    -----BEGIN CERTIFICATE-----
    ROOT CA
    -----END CERTIFICATE-----
    -----BEGIN CERTIFICATE-----
    INTERMEDIATE CA
    -----END CERTIFICATE-----
EOT
}
```

## Inputs

| Name | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `name` | `string` | Yes | n/a | Name of the Juju controller to bootstrap. |
| `cloud` | `object(...)` | Yes | n/a | Cloud definition used for bootstrap. |
| `cloud_credential` | `object(...)` | Yes | n/a | Credential used for bootstrap on the target cloud. |
| `controller_num_units` | `number` | Yes | n/a | Number of controller units to deploy. If greater than `1`, HA enablement runs. |
| `path_juju_binary` | `string` | No | `/snap/juju/current/bin/juju` | Path to Juju binary. |
| `agent_version` | `string` | No | `null` | Juju agent version to use. |
| `bootstrap_base` | `string` | No | `null` | Bootstrap base for controller machine. |
| `bootstrap_config` | `map(string)` | No | `null` | Bootstrap configuration values. |
| `bootstrap_constraints` | `map(string)` | No | `null` | Bootstrap constraints. |
| `controller_config` | `map(string)` | No | `null` | Controller configuration. |
| `controller_model_config` | `map(string)` | No | `null` | Controller model configuration. |
| `destroy_flags` | `object(...)` | No | `{ destroy_all_models = true, destroy_storage = true }` | Flags for `juju destroy-controller`. |
| `model_constraints` | `map(string)` | No | `null` | Constraints for all models. |
| `model_default` | `map(string)` | No | `null` | Default values for all models. |
| `storage_pool` | `object(...)` | No | `null` | Storage pool definition for the controller. |

## Outputs

| Name | Sensitive | Description |
| --- | --- | --- |
| `juju_cloud` | No | Cloud name used by the created controller. |
| `juju_controller` | Yes | Controller API addresses, username, password, and CA certificate. |

## Notes

- HA enablement is implemented with `terraform_data` + `local-exec` and Juju CLI commands as oppoed to Terraform actions to ensure compatibility with OpenTofu as actions are not yet supported. See https://github.com/opentofu/opentofu/issues/3309.
