---
name: tf-model-export-fixup
description: Take a `terraform query` export of a Juju model (after juju-tf-refwriter has been run) and iterate on `terraform plan` until the config both imports cleanly against the existing model (no changes, no replaces) AND would recreate every manually-created resource from scratch. Then split the single-file plan into maintainable modules. Use when the user has followed the import-manual-model how-to and wants an AI agent to finish refining the exported config.
applyTo: "**/*.tf"
---

# Terraform Model Export Fixup

The user has followed the
[import-manual-model how-to](https://github.com/juju/terraform-provider-juju/blob/main/docs-rtd/howto/import-manual-model.md):
they've run `terraform query`, `juju-tf-refwriter`, added a `required_providers`
block, and run `terraform init`. The working directory has `exported.tf` with
references resolved and a `.terraform.lock.hcl`.

The goal is a config that **both** imports cleanly (`terraform plan` → no
changes) **and** would recreate the model from scratch (remove `import` blocks
→ only creates). This is best-effort; some spurious diffs may remain and the
user may accept them.

## The loop

1. Run `terraform plan`. If the agent can't reach the controller, ask the user
   to run it and paste the output.
2. For each error or unexpected change, classify and fix it (see common cases
   below). Use `juju status --format yaml -m <model-name>` (or ask the user to
   run it) as the source of truth when the plan and the config disagree.
3. Re-run `terraform plan`. Repeat until `No changes` or the user accepts the
   remaining diffs.

**NEVER run `terraform apply`.** This skill takes a `.tf` file as input and
produces a `.tf` file as output. Applying would have surprising side-effects on
real infrastructure, and complex models need review before applying.

## Common cases

- **Unmanageable resources** (e.g. `juju_space` with `name = "alpha"`): remove
  the resource block and its `import` block together.
- **Controller-set defaults** (e.g. `juju_model.config` full of defaults): the
  export captures every value the controller reports. Look up Juju model config
  defaults, mark likely-default keys, and offer the user: (1) keep all, (2) drop
  likely defaults, or (3) case by case. Re-run the plan and repeat until clean
  or accepted.
- **Computed-only fields** (e.g. `storage`, `unit_numbers`): leave them out —
  don't add them to silence the diff.
- **`cloud` block on `juju_model`**: often controller-inferred, not user-set.
  Ask the user; if not set explicitly, remove it.
- **`juju_machine` resources / application `machines`**: often auto-allocated.
  Ask whether to drop them and let the provider allocate on recreate.
- **`juju_storage_pool` resources**: often implicit defaults. Ask whether to
  drop them or keep them explicit.
- **Resources to destroy ("not in configuration")**: expected when replacing an
  old config. Confirm with the user; don't re-add to prevent destruction. If a
  destroyed resource matches an imported one by identity (check `juju status`),
  suggest renaming the exported resource to the original label and dropping its
  `import` block so the existing state entry is reused.
- **`model_uuid` left as a literal**: the refwriter wasn't run or the model
  wasn't exported. Ask the user to re-run `terraform query` with the model
  included.

## When done

Once the plan is clean (or accepted), split for maintainability:

- Move `required_providers` into `versions.tf`.
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
may need manual review before applying.
