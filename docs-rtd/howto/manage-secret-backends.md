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

Secret backends are per model. To set which backend a model uses, set the `secret-backend` key in the model's config:

```terraform
resource "juju_model" "development" {
  name = "development"
  config = {
    secret-backend = juju_secret_backend.myvault.name
  }
}
```

```{important}
Changing the secret backend for a model does **not** migrate existing secrets to the new backend. Existing secrets remain stored in the backend they were originally created in. Only secrets created after the change will use the new backend.
```

## Migrate secrets to a new backend

If you want existing secrets to move to the new backend, you must force their replacement. Use the `replace_triggered_by` lifecycle directive to recreate secrets when the backend changes:

```terraform
resource "juju_secret" "my-secret" {
  model_uuid = juju_model.development.uuid
  name       = "my_secret"
  value = {
    key = "value"
  }

  lifecycle {
    replace_triggered_by = [juju_secret_backend.myvault.name]
  }
}
```

When the secret backend name changes, Terraform will destroy and recreate the secret, causing it to be stored in the new backend.

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
