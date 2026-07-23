## Description

Adds support for deploying a `juju_application` from a local `.charm` file, with out-of-band deploy detection for locally-deployed charms on supported Juju versions.

### Local charm deployment
- New `local_charm` block (mutually exclusive with `charm`): when set, the charm is uploaded from the local `.charm` file instead of being fetched from Charmhub. Applications can switch between `charm` and `local_charm` in place.
- New computed `local_charm.path_hash` (SHA-256 of the archive): detects when the local file content changes and drives an in-place charm refresh.
- `ValidateConfig` checks that `name` matches the charm name in the archive metadata, and that the configured `base` is compatible with the archive's declared bases.
- Changing the charm name in a new archive triggers a replace; changing only the content triggers an in-place refresh (`Update`).

### Out-of-band deploy detection (Juju >=3.6.26 and 4 only)
- New computed `local_charm.origin_hash`: the controller-reported charm hash from **`GetCharmURLOrigin`**. Empty on Juju <=3.6.25, which causes this detection to be disabled.
- On `Read`, if the controller's `origin_hash` no longer matches the value recorded at the last apply (for example someone ran `juju refresh` out of  band), the stale `path_hash` is invalidated. The next apply then recomputes the file hash and re-uploads the local charm, reconciling the controller back to the Terraform-managed charm.
- `path_hash` and `origin_hash` are independent fingerprints and are not compared against each other, so a future change to Juju's hashing scheme won't break this.

## Type of change

- Change existing resource
- Logic changes in resources (the API interaction with Juju has been changed)
- Change in tests (one or several tests have been changed)
- Requires a documentation update

## Manual QA

### 1. Build two slightly different `.charm` files

Make these files in `qa-local-charm/`

`metadata.yaml`
```yaml
name: qa-local
summary: minimal local charm for QA
description: minimal local charm for QA
```

`manifest.yaml`
```yaml
bases:
  - name: ubuntu
    channel: "22.04"
    architectures:
      - amd64
```

`dispatch` (required so the archive is accepted as a charm)
```sh
#!/bin/sh
```

Then zip the same files twice, with different `content`:

```sh
echo v1 > content
zip -q charm-v1.charm metadata.yaml manifest.yaml dispatch content

echo v2 > content
zip -q charm-v2.charm metadata.yaml manifest.yaml dispatch content
```

### 2. Deploy the local charm

The config always points at `charm-target.charm`; we swap which archive it links to between steps. This keeps the Terraform config byte-identical across the refresh, so the only thing that changes is the file content.

```sh
# Select charm v1
ln -sf charm-v1.charm charm-target.charm
```

```hcl
resource "juju_model" "qa" {
  name = "local-charm-qa"
}

resource "juju_application" "qa" {
  model_uuid = juju_model.qa.uuid
  name       = "qa-local"

  local_charm {
    name       = "qa-local"
    path       = "${path.module}/charm-target.charm"
    base       = "ubuntu@22.04"
  }
}
```

```sh
terraform apply
```

Verify:
- `terraform show` lists `local_charm.0.path_hash`.
- On Juju 4+, `local_charm.0.origin_hash` is also populated.
- `juju status` shows `qa-local` active.

### 3. Idempotency

```sh
terraform plan   # expect: No changes.
```

### 4. In-place refresh

```sh
# Select charm v2
ln -sf charm-v2.charm charm-target.charm
```

```sh
terraform plan    # expect: 1 to change (in-place update, not replace)
terraform apply
```

Verify `local_charm.0.path_hash` changed and the application was updated (not recreated).

### 5. Out-of-band drift reconciliation (disabled on Juju <=3.6.25)

Leave `path` pointing at the v2 archive in the config. Refresh the charm out of band, bypassing Terraform, with the v1 archive:

```sh
juju refresh qa-local --path ./charm-v1.charm
```

Then plan and apply with the unchanged (v2) config:

```sh
terraform plan
# expect: 1 to change. The plan shows local_charm.0.path_hash changing,
# because Read detected the origin_hash drift and invalidated the stale hash.

terraform apply
# Terraform re-uploads the v2 charm, reconciling the controller back to the
# Terraform-managed charm.

terraform plan
# expect: No changes. origin_hash is re-anchored to the deployed charm.
```

On Juju , step 5 shows `No changes` after the out-of-band refresh (`origin_hash` is empty, so drift detection is intentionally disabled).
