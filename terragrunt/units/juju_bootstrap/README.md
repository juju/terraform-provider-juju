# juju_bootstrap Terragrunt unit

This unit wraps the `modules/juju_bootstrap` Terraform module for use from a Terragrunt stack.

This definition expects to be used from within an stack. The enclosing stack is meant to provide a `values` attribute, which is used to populate the module source version, dependencies, required inputs, and any optional inputs.

## Expected stack-provided values

Required `values` entries:

- `version`
- `name`
- `cloud`
- `cloud_credential`
- `controller_num_units`

Optional `values` entries:

- `dependencies`
- `path_juju_binary`
- `agent_version`
- `bootstrap_base`
- `bootstrap_config`
- `bootstrap_constraints`
- `controller_config`
- `controller_model_config`
- `destroy_flags`
- `model_constraints`
- `model_default`
- `storage_pool`

## Behavior

- The module source is pinned from `values.version`.
- Terragrunt dependencies are populated from `values.dependencies` when present.
- Optional module inputs are forwarded only when the corresponding `values` entry is not `null`.

## Reference

For the definition of the forwarded inputs and module outputs, see `modules/juju_bootstrap/README.md`.