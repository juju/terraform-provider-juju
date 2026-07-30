---
name: tf-model-export-fixup
description: Take a `terraform query` export of a Juju model that was NOT deployed with Terraform, run juju-tf-refwriter over it, and iterate on `terraform plan` until the config both imports cleanly against the existing model (no changes, no replaces) AND would recreate every manually-created resource from scratch. Then split the single-file plan into maintainable modules. Use when the user has run `TF_VAR_model_uuid=... terraform query --generate-config-out=<file>.tf` on a hand-managed model and wants a Terraform config that reproduces it.
applyTo: "**/*.tf"
---

# Terraform Model Export Fixup

## Purpose

This skill converts a Juju model that was **not** deployed with Terraform into
a Terraform config that reproduces it. The starting point is `terraform query
--generate-config-out`, which emits a config describing the existing model with
`import` blocks and literal values for every field.

The end state is a config that satisfies **both** of these at once:

1. **Imports cleanly against the existing model** — `terraform plan` reports
   `No changes. Your infrastructure matches the configuration.` There are no
   replaces and no unexpected changes; every `import` block matches a real
   resource.
2. **Recreates the model from scratch** — if the `import` blocks are removed,
   `terraform plan` against a fresh model shows only `create` actions for every
   resource that was manually created. Nothing is missing, nothing drifts.

These are not two separate goals. A config that imports cleanly but would not
recreate the model is incomplete (it omits or misrepresents manually-created
resources). A config that recreates but does not import cleanly has drift or
wrong values. The skill is done only when both hold.

Out of the box the `terraform query` output does **not** satisfy either:

- No `terraform`/`required_providers` block, so `terraform init` fails with an
  inconsistent lock file or an unresolved provider source.
- Literal UUIDs and names instead of cross-resource references, so Terraform
  cannot order dependencies and the config cannot recreate the model. The
  `juju-tf-refwriter` tool fixes this mechanically.
- Resources that exist in every model but cannot be managed by the provider,
  e.g. the `alpha` system space, which fails validation with
  `Attribute name alpha is a system space and cannot be managed by juju_space`.
- Other provider-specific validation or drift issues that only surface at
  `terraform plan` time.

This skill guides an agent through the mechanical steps (`juju-tf-refwriter`,
editing the query file) and the judgment steps (reading plan errors and
patching `exported.tf`) until both goals hold, then splitting the result for
maintainability.

## Scope boundary — mechanical vs. judgment

Prefer the mechanical tool whenever it can do the job. The skill is only for
what the tools cannot do easily:

| Concern | Handled by |
| --- | --- |
| Rewriting literal UUIDs/names into resource references | `juju-tf-refwriter` |
| Filtering which resources are exported | the `.tfquery.hcl` file (edit the `list` blocks) |
| Adding the `terraform`/`required_providers`/`provider` block | this skill |
| Removing unmanageable resources (e.g. `alpha` space) | this skill (the `.tfquery.hcl` only supports inclusion filters, so it cannot exclude) |
| Resolving plan-time validation errors and drift | this skill |
| Splitting the single-file plan into maintainable files | this skill |

Do **not** duplicate `juju-tf-refwriter` behavior for the reference patterns
it already handles (`model_uuid`, application `machines`, integration
`application.name`). Run the tool. For reference patterns the refwriter does
**not** cover, hand-editing is allowed — but first consider whether the gap
could be handled deterministically by the refwriter. If it could, tell the user
to contact the maintainers about extending the tool rather than leaving it as a
per-export manual step.

## Prerequisites

The workspace must contain:

- A query file (e.g. `examples/list-resources/export-all-model.tfquery.hcl` or
  a copy in the working directory) listing every `list` block to export.
- A working directory with `exported.tf` (the `--generate-config-out` output)
  and a `.terraform.lock.hcl`. The provider is `juju/juju`.
- `juju-tf-refwriter/` — run with `go run . <path-to-exported.tf>` from that
  directory.

Confirm the controller is reachable and `TF_VAR_model_uuid` is set to the model
being exported before running `terraform plan`.

## Workflow

### 1. Run juju-tf-refwriter

This is the first step of the flow and the user has likely already run it by
the time they invoke an agent. Check whether the literals in `exported.tf`
have already been rewritten into references (e.g. `model_uuid =
juju_model.<label>.uuid` instead of a literal UUID). If so, skip this step.

Otherwise, from the `juju-tf-refwriter` directory:

```bash
go run . <absolute-or-relative-path-to>/exported.tf
```

This rewrites `model_uuid`, application `machines`, and integration
`application.name` literals into references. It prints warnings for any literal
it could not match; record those for step 4.

### 2. Ensure the provider block exists

`terraform query` does not emit a `terraform` block. Without it, `terraform
init` fails with an inconsistent lock file or an unresolved
`registry.terraform.io/hashicorp/juju` source.

Prepend this to `exported.tf` (or to a sibling `versions.tf`) if no
`required_providers` block is present anywhere in the working directory:

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

Pin the latest released major.minor (currently `~> 2.1`) unless the user asks
for a different constraint. The lock file records the exact resolved version.

If a `provider "juju"` block is needed for non-default auth (JAAS client
credentials, a specific controller), add it from the user's environment rather
than guessing. The provider reads `JUJU_CONTROLLER_ADDRESSES`,
`JUJU_USERNAME`, `JUJU_PASSWORD`, `JUJU_CA_CERT`, `JUJU_CLIENT_ID`, and
`JUJU_CLIENT_SECRET` from the environment by default, so a provider block is
often unnecessary.

### 3. Remove unmanageable resources from exported.tf

The `.tfquery.hcl` `list` schemas only support **inclusion** filters (e.g.
`name` on `juju_space`), not exclusion, so they cannot prevent resources like
the `alpha` system space from being exported. Such resources must be removed
from `exported.tf` **and** their matching `import` block. This step covers the
*known* cases; other unmanageable resources are discovered reactively in step 4
when the plan rejects them. Today the known cases include:

- `juju_space` with `name = "alpha"` — the system space; the provider rejects
  it with `Attribute name alpha is a system space and cannot be managed by
  juju_space`.

Delete both the `resource "juju_space" ...` block and the immediately following
`import { to = juju_space.<same label> ... }` block. Leave user-created spaces
untouched.

Apply the same pattern to any other resource the plan rejects as unmanageable.

### 4. Iterate on terraform plan until both goals hold

Run, from the working directory containing `exported.tf`:

```bash
terraform init
terraform plan
```

It may not be practical for the agent to run `terraform plan` itself — it needs
a reachable Juju controller, valid credentials, and the model to exist. Attempt
to run it, but if it fails for environment reasons (no controller, auth errors,
model not found, network blocked), do not loop on retries. Ask the user to run
`terraform plan` and paste the output, then classify and act on what they
report.

For each error or unexpected change, classify and act:

- **Validation error on a specific resource** (e.g. a system space, a
  read-only field, a value the provider refuses): remove the resource or
  field per step 3, or correct the value. Prefer removing over patching when
  the field is not user-manageable.
- **`model_uuid` left as a literal** (refwriter warning): check that a
  `juju_model` resource with a matching `uuid` exists in the file. If the
  model itself was not exported, the reference cannot be formed; add the model
  to the query file and re-export.
- **Drift / "1 to replace" or "1 to change"**: read the diff. Common causes:
  - A field the provider cannot round-trip (e.g. a default the controller set
    that the provider would not set on create). Align the config to the real
    value so that both import and recreate are clean.
  - `config = null` vs `{}` on resources that default to empty maps. Set the
    field to the empty collection if the provider treats `null` and empty
    differently.
  - **Controller-set defaults captured by the export** (common). The export
    captures every value the controller reports, including defaults the
    original creation never specified. The clearest case is `juju_model.config`:
    a model created with no `config` block still exports a full map of
    controller defaults (`agent-stream`, `logging-config`, `firewall-mode`,
    `lxd-snap-channel`, etc.). On import these show as drift (the state has
    them, the config sets them to the same or `null` values); on recreate they
    duplicate the defaults the controller would apply anyway. The same applies
    to `annotations = {}` and to default-valued attributes on other resources.
    The agent will not have the original config to compare against, so it
    cannot know directly which values the user set vs. which the controller
    defaulted. The user may know in some cases, but typically won't — since
    the model was not made with Terraform, there is no config of record to
    consult, and the user may not remember what they set by hand. To help
    classify, look up the Juju model config defaults (from the Juju docs or
    the provider schema) and mark each exported value as "likely default" when
    it matches the documented default, or "non-default" when it differs. When
    presenting the case-by-case choice, pre-select the likely-default keys for
    dropping and the non-default keys for keeping, and let the user confirm or
    override. Treat a value as a controller default when it matches the
    provider's documented default for that field. When a value is confirmed as
    a default the user did not set, remove it from the config so it reproduces
    what the user actually specified, not the controller's defaults. Confirm
    against the provider schema that a field is defaulted (not required)
    before removing it. If removing a whole map like `config`, drop the
    attribute entirely rather than leaving it `null` or `{}` unless the plan
    shows the provider needs the empty collection.

    For a captured-defaults map like `juju_model.config`, the goal is a plan
    with 0 changes. Show the user the exported map with the likely-default keys
    marked, and offer: (1) keep all keys as exported (safe default — no changes
    to the config block), (2) "accept likely defaults" — drop the likely-default
    keys and keep the rest, or (3) decide case by case. If the user picks (2),
    apply it, re-run the plan, and if any changes remain for this resource, ask
    again about the remaining keys — repeat until there are no changes or the
    user says to accept the remaining changes. If the user picks (3), present
    the keys in groups (or one at a time, if the set is small) and ask which to
    keep vs. drop, then apply the selection and re-run the plan; continue
    asking about any remaining changes until none remain or the user accepts
    them.
  - **Computed-only fields the export omitted** (e.g. `storage`,
    `storage_directives`, `unit_numbers` on `juju_application`). The plan
    shows them as `+` (known after apply) or `~` against a current value. If
    the field is computed and not settable, leave it out of the config — do
    not add it to silence the diff. If the export set it to a literal that
    differs from the real value, remove the literal.
  - **`timeouts` sub-blocks** on resources like `juju_machine`. The export
    emits `timeouts { create = "30m" }`, which the plan may show as `+` on
    import. Keep the block if the provider accepts it on create; remove it if
    it only surfaces as drift on import.
  - **`cloud` block on `juju_model`**. The export emits a `cloud { name ...
    region ... }` block derived from the model's hosting cloud. Many users do
    not set this explicitly — the controller infers it from the model's
    cloud/region at creation. If the user did not set `cloud` in their original
    config, keeping it can cause drift or force a recreate. Ask the user
    whether they set `cloud` explicitly; if not, remove the block so the
    provider infers it on recreate the same way it was inferred originally.
  - **`juju_machine` resources and application `machines`**. It is common for
    applications to be created without explicitly pinning machines — the
    controller allocates machines automatically. The export captures those
    auto-allocated machines as `juju_machine` resources and pins each
    `juju_application`'s `machines` list to them. If the user did not pin
    machines in their original config, the exported `juju_machine` resources
    and the `machines` attributes on applications are controller-allocated
    artifacts, not user intent. Ask the user whether it is acceptable for
    `juju_application` to be the only thing controlling the machine indirectly
    (i.e. drop the `juju_machine` resources and the `machines` lists, letting
    the provider allocate machines on recreate). Ask whether they want this
    decision applied per-application or as a blanket rule across the config.
  - **`juju_storage_pool` resources**. It is common for a model to have an
    implicit storage pool (e.g. the default `lxd` pool on LXD clouds) that the
    controller creates automatically. The export captures these as explicit
    `juju_storage_pool` resources. If the user did not define storage pools in
    their original config, these are controller-provided defaults, not user
    intent. Ask the user whether they want the storage pool to remain implicit
    (drop the `juju_storage_pool` resources so the controller provides it on
    recreate) or be explicit (keep them). Apply the decision as a blanket rule
    unless the user asks for per-pool control.
- **Resources to destroy ("not in configuration")**: the plan lists resources
  for destruction because they exist in state (from a prior `apply` of a
  different config) but not in `exported.tf`. The agent will not have the
  prior config to compare against. This is expected when the user is replacing
  an old config with the export. Confirm with the user that those resources
  should indeed be removed from state — do not add them to `exported.tf` to
  prevent destruction unless the user wants them kept. The goal is a config
  that reproduces the *model*, not one that preserves the previous Terraform
  state.

  A common sub-case (typically encountered when developing this skill itself):
  the model was *originally* created with Terraform, and the user is now
  regenerating the config with `terraform query`. The export
  produces new resources with auto-generated labels (e.g. `juju_model.model_0`,
  `juju_application.all_apps_0`) that may refer to the same real-world objects
  as the original hand-written resources (e.g. `juju_model.test`,
  `juju_application.source`). The plan then shows the originals as "to
  destroy" and the exports as "to import" — which, if they are the same
  objects, would destroy and recreate the same infrastructure. When you see a
  destroy for a resource whose real identity matches an imported resource in
  `exported.tf`, figure out the rename (match by `id`/identity, not by label)
  and suggest it to the user: rename the exported resource to the original
  label and drop its `import` block so the existing state entry is reused. The
  ideal outcome is that the plan shows 0 to create, 0 to destroy, 0 to change
  for those resources — they are already in state under the original label, so
  reusing that label means nothing needs to happen to them. If the user
  accepts, apply the rename. If the user declines, leave the export as-is and
  let the originals be removed from state.
- **Import identity mismatch**: the `import` block's `identity.id` must match
  the resource's real identity. `terraform query` generates these correctly;
  only edit if the resource was renamed.

Re-run `terraform plan` after each fix. When the plan reports `No changes.
Your infrastructure matches the configuration.`, tell the user they are
likely done with this step — but remind them that the entire process is
best-effort: the resulting config may not correctly match the infrastructure,
and the user should check it manually as well before relying on it.

The skill is done only when **both** checks pass:

1. **Import check** — with `import` blocks present, `terraform plan` reports
   `No changes. Your infrastructure matches the configuration.` No replaces,
   no unexpected changes.
2. **Recreate check (best effort)** — the agent may run `terraform plan` against
   existing infrastructure, but MUST NOT create any new resources (no fresh
   model, no `apply`). So the recreate check is best-effort: remove (or comment
   out) all `import` blocks and remove the literal `uuid` attribute from the
   `juju_model` resource, then reason about whether each resource has a
   complete create path — every required attribute is set, every dependency is
   referenced, nothing relies on a value only the controller would supply. If a
   fresh model is available and the user is willing to run it, ask the user to
   run `terraform plan` against it and report whether the only changes are
   `create` actions in dependency order. Otherwise, flag any resource whose
   create path you cannot confirm and ask the user to verify.

   Restore the `import` blocks and the `uuid` attribute afterwards so the
   config still imports cleanly against the original model.

NEVER run `terraform apply` even if the user explicitly asks. Tell the user
that it's unsafe for an agent to do so and the user should run it themselves
if they want it done.

### 5. Split the plan for maintainability

Once both goals hold, the config is correct but lives in a single generated
file with auto-generated labels like `all_apps_0`. Split it into a structure
that is easier to maintain:

- Move the `terraform`/`required_providers` block into `versions.tf`.
- Group resources into separate files by concern (e.g. `model.tf`,
  `machines.tf`, `applications.tf`, `integrations.tf`, `secrets.tf`,
  `offers.tf`, `spaces.tf`, `storage_pools.tf`, `ssh_keys.tf`).
- Rename auto-generated resource labels to meaningful names that match the
  model's real entities (e.g. `juju_application.all_apps_0` →
  `juju_application.source`). Update every reference and the matching `import`
  block's `to` in the same edit.
- Collect all `import` blocks into a single `imports.tf`. This is common
  practice: the resource definitions live in their concern files, while every
  `import` block — one per resource — lives together in `imports.tf`. Keep the
  `to` addresses in `imports.tf` in sync with any resource renames done above.
- Remove all generated artifacts so the result reads as if written from scratch
  by a human: delete `# __generated__ by Terraform` headers, the
  `# Please review these resources and move them into your main configuration
  files.` notice, and any other `terraform query` boilerplate comments. Drop
  redundant `provider = juju` attributes on resources (the provider is already
  declared globally). Only remove empty/`null` attributes that are confirmed
  redundant — confirmed by the provider schema, reported by the user, or
  shown to be redundant by the plan. Do not remove attributes on the assumption
  that they match a default you have not checked.
- Re-run `terraform plan` after the split and renames to confirm both goals
  still hold. Renames change addresses, so the import check is the real test
  that nothing was broken.

## Conventions

- Keep all edits inside the working directory's `.tf` files. Do not modify
  `juju-tf-refwriter` source as part of this skill; if the refwriter misses a
  reference pattern, note it as a follow-up rather than patching the tool
  inline.
- Remove `# __generated__ by Terraform` and other `terraform query` boilerplate
  during the split (step 5), not earlier — keep them while iterating on the
  plan so diffs stay comparable to the original export.
- Do not reformat the file beyond the edits needed until the final split; the
  user may diff against the original `terraform query` output.
- When removing a resource, remove its `import` block in the same edit to
  avoid a dangling import that fails `terraform plan`.

## Export coverage limitation

The `.tfquery.hcl` only lists resources with `list` support. Anything else in
the model — users, credentials, access grants, JAAS resources, and any other
resource type the provider exposes without a `list` implementation — will not
appear in `exported.tf`. The resulting config therefore reproduces only the
exported subset. Before declaring the skill done, tell the user which resource
types were not exported and that they may have to recreate those manually, even
after the agent is finished.

Also tell the user that the agent's config is a best-effort reproduction of the
model. It's possible it won't entirely match and the user may have to change it
further.

## When to stop and ask

- A plan diff cannot be explained by the cases above and would require
  changing provider behavior or the refwriter.
- A resource the user cares about is unmanageable by the provider and there is
  no query-file filter to exclude it; confirm before deleting it from the
  config.
- The two goals conflict for a specific resource (e.g. a value that imports
  cleanly but would not recreate, or vice versa) and no single config value
  satisfies both.
- The user asks for a change that would break one of the two goals (e.g. changing
  a field to a value that would not import cleanly).
- Any time there is lack of clarity about the user's intent or the model's state.
  ALWAYS ask for clarification rather than guessing.
