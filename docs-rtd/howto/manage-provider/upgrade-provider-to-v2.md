---
myst:
  html_meta:
    description: "Complete guide for upgrading the Terraform Provider for Juju from v1.x to v2.0.0."
---

(upgrade-to-terraform-provider-juju-v-2)=
# Upgrade the provider to v2

The v2 version of the Terraform provider:

- Doesn't introduce any breaking changes to resources.
- Adds support for Juju 4.x controllers.
- Drops support for Juju 2.9 controllers.

## Before you upgrade

Check which version of Juju your controller is running before deciding whether to upgrade.

- **Juju 2.9 controllers are not supported in v2.** If you are using v1 against a Juju 2.9 controller, do not upgrade to v2. We will keep supporting 2.9 controller in the v1 track for security and bug fixes.
- **Juju 3.x controllers:** Upgrade to the latest v1 release first, then upgrade to v2.
- **Juju 4.x controllers:** v2 is the first provider release to support Juju 4.

## Upgrade steps

1. Update the provider version constraint in your Terraform configuration:

   ```hcl
   terraform {
     required_providers {
       juju = {
         source  = "juju/juju"
         version = "~> 2.0"
       }
     }
   }
   ```

2. Run `terraform init -upgrade` to download the new provider version.
3. Run `terraform plan` to review any changes before applying.

