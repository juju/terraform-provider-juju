# juju-tf-refwriter

A command-line tool that rewrites the Terraform configuration produced by
`terraform query` so that literal UUIDs and names become references to the
resources defined in the same file.

## Why

`terraform query --generate-config-out` (see
[Export a model](../docs-rtd/howto/manage-models.md)) emits a plan where every
field is a literal value. For example, an application's `model_uuid` is a
hard-coded UUID rather than a reference to the `juju_model` resource, and an
integration's `application` blocks use literal application names instead of
references to the `juju_application` resources.

This works for importing existing infrastructure, but the plan can't be reused
to reproduce the same setup elsewhere: Terraform can't see the dependency
between the model and the application, so it tries to create both at once and
fails.

`juju-tf-refwriter` fixes this by rewriting the literals into references and
pruning attributes that `terraform query` emits but that can't or shouldn't
be set in config:

| Literal | Reference / action |
| --- | --- |
| `model_uuid = "<uuid>"` | `model_uuid = juju_model.<label>.uuid` |
| `machines = ["1"]` | `machines = [juju_machine.<label>.machine_id]` |
| integration `application { name = "app" }` | `application { name = juju_application.<label>.name }` |
| `<attr> = null` | removed (the provider default is already `null`) |
| a Computed-only attribute (e.g. model `uuid`, machine `machine_id`) | removed (cannot be set in config) |

The tool reads the resource identities from the `import` blocks that
`terraform query` always emits, so it can match literals to the right resource
even when the resource block itself has no `id` attribute.

## Usage

Rewrite a single file in place:

```bash
go run github.com/juju/terraform-provider-juju/juju-tf-refwriter path/to/test.tf
```

Rewrite all `.tf` files in a directory in place:

```bash
go run github.com/juju/terraform-provider-juju/juju-tf-refwriter path/to/terraform/directory
```

A typical workflow, continuing from
[Export a model](../docs-rtd/howto/manage-models.md):

```bash
TF_VAR_model_uuid="<model-uuid>" terraform query --generate-config-out=test.tf
go run github.com/juju/terraform-provider-juju/juju-tf-refwriter test.tf
terraform plan
```

## Example

**Before** (excerpt of `terraform query` output):

```terraform
resource "juju_model" "model_0" {
  uuid = "c1cecf1e-fe66-4589-8585-e579edd6f58b"
}

import {
  to       = juju_model.model_0
  provider = juju
  identity = {
    id = "c1cecf1e-fe66-4589-8585-e579edd6f58b"
  }
}

resource "juju_application" "all_apps_0" {
  config     = null
  machines   = ["1"]
  model_uuid = "c1cecf1e-fe66-4589-8585-e579edd6f58b"
  name       = "dummy-sink"
}

import {
  to       = juju_application.all_apps_0
  provider = juju
  identity = {
    id = "c1cecf1e-fe66-4589-8585-e579edd6f58b:dummy-sink"
  }
}

resource "juju_machine" "all_machines_0" {
  model_uuid = "c1cecf1e-fe66-4589-8585-e579edd6f58b"
  machine_id = "1"
}

import {
  to       = juju_machine.all_machines_0
  provider = juju
  identity = {
    id = "c1cecf1e-fe66-4589-8585-e579edd6f58b:1:machine-1"
  }
}
```

**After:**

```terraform
resource "juju_application" "all_apps_0" {
  machines = [
    juju_machine.all_machines_0.machine_id
  ]
  model_uuid = juju_model.model_0.uuid
  name       = "dummy-sink"
}

resource "juju_machine" "all_machines_0" {
  model_uuid = juju_model.model_0.uuid
}
```

Note that `juju_model.model_0`'s `uuid` and `juju_machine.all_machines_0`'s
`machine_id` are removed because they are Computed (cannot be set in config),
and `config = null` is removed because the default is already `null`.

## What is left alone

- `model_uuid` values that are already references (e.g.
  `model_uuid = juju_model.m.uuid`) are not touched.
- `model_uuid` literals with no matching `juju_model` resource in the file are
  left as literals and a warning is printed, so you can review them manually.
- Attributes that are both `Computed` and `Optional` (e.g. an application's
  `name`) are kept, since the user may set them. Only Computed-only
  attributes are pruned. The list of pruned attributes is maintained by hand
  in `computed.go`; if you add or change a Computed-only attribute in the
  provider schema, update that list too.
- Default/implicit resources emitted by `terraform query` (such as the `loop`
  storage pool) are kept; only their references are rewritten. Remove them
  manually if you don't want to manage them with Terraform.

## Testing

```bash
go test -v
```

The `in/` directory holds input fixtures and `out/` holds the expected
rewritten output; `TestRewriteTransformation` compares the two.
