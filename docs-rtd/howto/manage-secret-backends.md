---
myst:
  html_meta:
    description: "Learn how to manage secret backends in Juju models using the Terraform Provider for Juju."
---

(manage-secret-backends)=
# Manage secret backends

> See also: {external+juju:ref}`Juju | Secret backend <secret-backend>`

## Add a secret backend

To add a secret backend to the controller, in your Terraform plan create a resource of the `juju_secret_backend` type, specifying a name, the backend type, and the backend configuration. For example, to add a Vault backend:

```terraform
resource "juju_secret_backend" "myvault" {
  name         = "myvault"
  backend_type = "vault"
  config_wo = {
    endpoint = "https://vault.example.com:8200"
    token    = "s.exampletoken"
  }
  config_wo_version = 1
}
```

```{note}
The `config_wo` attribute is write-only — its content is never stored in Terraform state. To update the configuration, change the `config_wo` values and bump `config_wo_version`.
```

> See more: [`juju_secret_backend` (resource)](../reference/terraform-provider/resources/secret_backend)

## Set the secret backend for a model

Secret backends are per model. The recommended way to set which backend a model uses is the `secret_backend` attribute on the model resource:

```terraform
resource "juju_model" "development" {
  name           = "development"
  secret_backend = juju_secret_backend.myvault.name
}
```

```{note}
On Juju 4+, this uses the dedicated `model-secret-backend` API. On Juju 3, it falls back to setting the `secret-backend` model config key. When using this attribute, the `secret-backend` key is stripped from the model's `config` attribute to avoid state drift.
```

For backward compatibility, you can also set the `secret-backend` key directly in the model's `config` block. This works on Juju 3 but is not recommended on Juju 4, where the `secret_backend` attribute should be used instead:

```terraform
resource "juju_model" "development" {
  name = "development"
  config = {
    secret-backend = juju_secret_backend.myvault.name
  }
}
```

```{important}
Changing the secret backend for a model triggers an **asynchronous migration** of existing secrets to the new backend. This applies in both directions — from one external backend (e.g. Vault) to another, and from the default `internal` backend to an external backend.

Because the migration is asynchronous, the `juju_model` update returns before the secrets have finished moving. A `juju_secret` inspected immediately after the change may therefore still report the *old* backend.
```

## Update a secret backend

To update a secret backend's configuration, change the `config_wo` values and bump `config_wo_version`:

```terraform
resource "juju_secret_backend" "myvault" {
  name         = "myvault"
  backend_type = "vault"
  config_wo = {
    endpoint = "https://vault.example.com:8200"
    token    = "s.newtoken"
  }
  config_wo_version = 2
}
```

## Remove a secret backend

To remove a secret backend, remove its resource definition from your Terraform plan.

> See more: [`juju_secret_backend` (resource)](../reference/terraform-provider/resources/secret_backend)
