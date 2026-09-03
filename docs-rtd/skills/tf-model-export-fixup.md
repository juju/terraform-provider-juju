---
name: tf-model-export-fixup
description: Take a `terraform query` export of a Juju model and turn it into a clean, maintainable Terraform config. Rewrite literal UUIDs and names into cross-resource references, prune attributes that can't or shouldn't be set, then iterate on `terraform plan` until the config both imports cleanly against the existing model (no changes, no replaces) AND would recreate every resource from scratch. Finally split the single-file plan into maintainable modules.
---

# Terraform model export fixup

The user has a `terraform query` export of a Juju model (`exported.tf`), generated with
`terraform query --generate-config-out`. Every field in it is a literal value, and it
has no `terraform`/`required_providers` block yet. The goal is a config that **both**
imports cleanly (`terraform plan` → no changes) **and** would recreate the model from
scratch (remove `import` blocks → only creates). This is best-effort; some spurious
diffs may remain and the user may accept them.

**NEVER run `terraform apply`.** This skill takes a `.tf` file as input and produces a
`.tf` file as output. Applying would have surprising side-effects on real
infrastructure, and complex models need review before applying.

## 1. Set up the config for `terraform plan`

The generated file has no `terraform`/`required_providers` block. Add one in a sibling
`versions.tf`:

```terraform
terraform {
  required_providers {
    juju = {
      source  = "juju/juju"
      version = "~> 2.1"
    }
  }
}
```

Then run `terraform init`.

## 2. Rewrite literals into references

`terraform query` emits every field as a literal: `model_uuid` is a hard-coded UUID,
an application's `machines` list holds literal machine IDs, and an integration's
`application` blocks use literal application names. Rewrite them so Terraform can see
the dependency graph:

| Literal | Reference |
| --- | --- |
| `model_uuid = "<uuid>"` | `model_uuid = juju_model.<label>.uuid` |
| `machines = ["1"]` | `machines = [juju_machine.<label>.machine_id]` |
| integration `application { name = "app" }` | `application { name = juju_application.<label>.name }` |

To match a literal to the right resource, use the `import` blocks, which
`terraform query` always emits and which carry the composite identity in
`identity.id`:

- `juju_model`: the `id` is the model UUID.
- `juju_application`: `<model-uuid>:<app-name>`
- `juju_machine`: `<model-uuid>:<machine-id>:<machine-name>`
- `juju_integration`: `<model-uuid>:<app1>:<ep1>:<app2>:<ep2>`

So, for example, an application whose `import` identity is
`<model-uuid>:dummy-sink` matches the `juju_application` block whose `name` is
`dummy-sink` in the model with that UUID, and a `machines = ["1"]` entry matches the
`juju_machine` whose identity has machine ID `1` in the same model.

Rules:

- `model_uuid` values that are already references are left untouched.
- `model_uuid` literals with no matching `juju_model` resource in the file are left
  as literals; warn the user and suggest re-running `terraform query` with the model
  included.
- Default/implicit resources emitted by `terraform query` (such as the `loop` storage
  pool) are kept; only their references are rewritten.

## 3. Prune attributes that can't or shouldn't be set

- **`null` attributes**: `terraform query` emits every Optional attribute as `null`
  when unset. Remove them; the provider default is already `null`.
- **Computed-only attributes** (Computed, not Optional or Required) cannot be set in
  config and fail validation on apply. Remove them. Common ones:
  - `juju_model`: `uuid`, `id`
  - `juju_application`: `id`, `model_type`, `unit_numbers`, `storage`
  - `juju_machine`: `id`, `machine_id`, `instance_id`, `hostname`
  - `juju_secret`: `id`, `secret_id`, `secret_uri`
  - `juju_integration`, `juju_access_model`, `juju_access_secret`: `id`
- Attributes that are both Computed and Optional (e.g. application `name`) are kept;
  the user may set those.

## 4. Iterate on `terraform plan`

1. Run `terraform plan`. If you can't reach the controller, ask the user to run it
   and paste the output.
2. For each error or unexpected change, classify and fix it (see common cases
   below). Use `juju status --format yaml -m <model-name>` (or ask the user to run
   it) as the source of truth when the plan and the config disagree.
3. Re-run `terraform plan`. Repeat until `No changes` or the user accepts the
   remaining diffs.

Common cases:

- **Unmanageable resources** (e.g. `juju_space` with `name = "alpha"`): remove the
  resource block and its `import` block together.
- **Controller-set defaults** (e.g. `juju_model.config` full of defaults): the export
  captures every value the controller reports. Look up Juju model config defaults,
  mark likely-default keys, and offer the user: (1) keep all, (2) drop likely
  defaults, or (3) case by case.
- **`cloud` block on `juju_model`**: often controller-inferred, not user-set. Ask
  the user; if not set explicitly, remove it.
- **`juju_machine` resources / application `machines`**: often auto-allocated. Ask
  whether to drop them and let the provider allocate on recreate.
- **`juju_storage_pool` resources**: often implicit defaults. Ask whether to drop
  them or keep them explicit.
- **Resources to destroy ("not in configuration")**: expected when replacing an old
  config. Confirm with the user; don't re-add to prevent destruction. If a destroyed
  resource matches an imported one by identity (check `juju status`), suggest
  renaming the exported resource to the original label and dropping its `import`
  block so the existing state entry is reused.

## 5. When done

Once the plan is clean (or accepted), split for maintainability:

- Move `required_providers` into `versions.tf` if not already there.
- Group resources by concern into separate files.
- Rename auto-generated labels to meaningful names. Update all references and
  `import` block `to` addresses in the same edit.
- Collect all `import` blocks into `imports.tf`.
- Remove `# __generated__` headers and other boilerplate. Drop redundant
  `provider = juju` attributes. Only remove `null`/empty attributes confirmed
  redundant by the schema, the user, or the plan.
- Run `terraform fmt -recursive`, then `terraform plan` once more to confirm.

Tell the user: the export only covers resources with `list` support (users,
credentials, access grants, etc. won't appear). The config is best-effort and
**NEEDS** careful manual review before applying.
