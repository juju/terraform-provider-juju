(upgrade-to-terraform-provider-juju-v-1)=
# Upgrade the provider to v1

This guide shows how to modify your plans when upgrading from pre-`v1.0.0` versions of the Terraform Provider for Juju to `v1.0.0`.

Version `1.0.0` is a major release with breaking changes that requires changes to your plans. Upgrades from any `v0.x` version
are supported.

## Breaking changes

### 1. Model name replacement

The most significant change is the replacement of the `model` field with `model_uuid` across multiple resources and data sources.

**Affected resources:**
- `juju_application`
- `juju_offer`
- `juju_ssh_key`
- `juju_access_model`
- `juju_access_secret`
- `juju_integration`
- `juju_secret`
- `juju_machine`

**Affected data sources:**
- `juju_model`
- `juju_application`
- `juju_secret`
- `juju_machine`

For example:

**Before v1.0.0:**
```terraform
resource "juju_application" "app" {
  name  = "postgresql"
  model = juju_model.test.name
  charm {
    name = "postgresql"
  }
}

data "juju_model" "existing" {
  name = "my-model"
}
```

**After `v1.0.0`:**
```terraform
resource "juju_application" "app" {
  name       = "postgresql"
  model_uuid = juju_model.test.uuid # <--
  charm {
    name = "postgresql"
  }
}

data "juju_model" "existing" {
  uuid = "model-uuid-here" # <--
}
```

This change does not require resource re-creation. All infrastructure should remain intact
and only the Terraform state will be updated.

### 2. Import syntax changes

Import syntax for model-scoped resources has changed to require model UUIDs instead of model names.

**Before `v1.0.0`:**
```bash
terraform import juju_application.myapp model-name:application-name
```

**After `v1.0.0`:**
```bash
terraform import juju_application.myapp model-uuid:application-name
```

### 3. Offer data source changes

The `juju_offer` data source no longer contains the computed `model` field.

### 4. Application resource

Several deprecated fields have been removed from the `juju_application` resource:

- **`placement`** - Use `machines` instead. This is expected to cause resource replacement.
- **`principle`** - Field was unused and has been removed.
- **`series`** - Use `base` instead.

### 5. Machine resource

The deprecated `series` field has been removed from the `juju_machine` resource. Use `base` instead.

**Before v1.0.0:**
```terraform
resource "juju_machine" "machine" {
  model  = juju_model.test.name
  series = "focal"
}
```

**After v1.0.0:**
```terraform
resource "juju_machine" "machine" {
  model_uuid = juju_model.test.uuid
  base       = "ubuntu@20.04"
}
```

## Automated upgrade tool

The team provides an automated upgrade tool called `tf-upgrader` to help migrate your Terraform configurations.

### Using `tf-upgrader`

The tool can be run directly using Go:

```bash
# Upgrade a single file
go run github.com/juju/terraform-provider-juju/tf-upgrader path/to/file.tf

# Upgrade all .tf files in a directory
go run github.com/juju/terraform-provider-juju/tf-upgrader path/to/terraform/directory
```

### What `tf-upgrader` does

The tool automatically:

1. Transforms resources and data sources that reference `model = juju_model.*.name` to `model_uuid = juju_model.*.uuid`.
2. Transforms output blocks that reference `juju_model.*.name` to `juju_model.*.uuid`.
3. Updates your plan/module's `required_providers` block to specify a minimum Juju provider version of `1.0.0`.
4. Issues a warning for scenarios that require manual intervention.

### What `tf-upgrader` won't upgrade

The tool cannot automatically upgrade:

- Resources that reference variables (e.g., `model = var.model_name`).
- Resources that reference hardcoded strings (e.g. `model = "stg-model"`).
- Complex expressions or conditional logic.
- Resources without model references.
- The tool will not add a minimum provider version if one is not specified (it will only issue a warning instead).

The tool will show warnings for variables containing "model" in their name, as these may need manual review.

## Upgrade steps

### Step 1: Back up your configuration

This step is recommended but not required.

Before making any changes:

```bash
# Backup your Terraform files (or use version control)
cp -r your-terraform-config your-terraform-config-backup

# Backup your Terraform state
terraform state pull > terraform.tfstate.backup
```

### Step 2: Upgrade provider version

We recommend using [version constraints](https://developer.hashicorp.com/terraform/language/providers/requirements#provider-versions)
to specify your provider version.

**Example**

**Before v1.0.0:**
```terraform
terraform {
  required_providers {
    juju = {
      source = "juju/juju"
      version = "0.20.0"
    }
  }
}
```

**After v1.0.0:**
```terraform
terraform {
  required_providers {
    juju = {
      source = "juju/juju"
      version = "~> 1.0" # Allows any 1.x version
    }
  }
}
```

### Step 3: Run `tf-upgrader`

```bash
go run github.com/juju/terraform-provider-juju/tf-upgrader .
```

Check the output for any warnings that will indicate fields
that require further inspection.

### Step 4: Review and update variables

Check for any variables that reference model names:

```terraform
# Before - needs manual update
variable "model_name" {
  description = "The name of the model"
  type        = string
}

resource "juju_application" "app" {
  model = var.model_name  # This won't be auto-upgraded
}

# After - manual update required
variable "model_uuid" {
  description = "The UUID of the model"
  type        = string
}

resource "juju_application" "app" {
  model_uuid = var.model_uuid
}
```

### Step 5: Update import statements

If you use `terraform import`, update your import commands to use UUIDs:

```bash
# Get the model UUID first
juju models --format=json | jq -r '.models[] | select(.name=="your-model") | ."model-uuid"'

# Use the UUID in import commands
terraform import juju_application.myapp model-uuid:application-name
```

### Step 6: Plan and apply

After making changes:

```bash
# Initialize with the new provider version
terraform init -upgrade

# Review the planned changes
terraform plan

# Apply the changes
terraform apply
```

In most cases where the only change is to move from model name to model UUID, expect no resource recreation.

## Validation

After upgrading, verify your configuration:

1. Run your Terraform plan - it should show no errors and all the expected changes.
2. Check resource state - verify resources are correctly referenced by UUID

## Getting help

If you encounter issues during the upgrade:

1. Check the docs here or on the [Terraform registry](https://registry.terraform.io/providers/juju/juju/latest/docs).
2. Review the [changelog](https://github.com/juju/terraform-provider-juju/blob/main/CHANGELOG.md) for detailed change information.
3. File issues on the [GitHub repository](https://github.com/juju/terraform-provider-juju/issues).
4. Join the [Juju community on Matrix](https://matrix.to/#/#terraform-provider-juju:ubuntu.com).
