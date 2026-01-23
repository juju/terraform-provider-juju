---
myst:
  html_meta:
    description: "Learn how to deploy Charmhub charms with specific channels and revisions, including automatic revision computation examples."
---

(manage-charms)=
# Manage charms

> See also: {external+juju:ref}`Juju | Charm <charm>`

(deploy-a-charm)=
## Deploy a charm

The Terraform Provider for Juju does not support deploying a local charm.

To deploy a Charmhub charm, in your Terraform plan add a `juju_application` resource, specifying the target model and the charm you want to deploy:

```terraform
resource "juju_application" "this" {
  model_uuid = juju_model.development.uuid

  charm {
    name = "hello-kubecon"
  }
}
```

You can also specify a charm channel and revision. For example:


```terraform
resource "juju_application" "this" {
    model = <model>
    charm {
        name     = "<charm-name>"
        channel  = "<channel-name>"
        revision = "<revision-number>"
    }
}
```

This works as follows:

- If both `channel` and `revision` are specified (recommended for reproducibility), the Terraform provider will deploy the requested revision.
- If only `channel` is specified, the provider will deploy the latest revision available in that channel. The charm will not be refreshed on subsequent `terraform apply` runs.
- If only `revision` is specified, the provider will try to deploy that revision from the default channel (as set for the charm on Charmhub); if not available, the result will be an error.
- If neither field is specified, the provider will deploy the latest revision from the default channel (as set for the charm on Charmhub). The charm will not be refreshed on subsequent `terraform apply` runs.

If the charm has any resources, and your Terraform plan does not specify them explicitly, resources will come from the tip of the specified or inferred channel.

> See more: [`juju_application` (resource)](../reference/terraform-provider//resources/application)

(compute-a-charms-revision-automatically)=
### Compute a charm's revision automatically

The Terraform Provider for Juju requires you to specify both the charm `channel` and the charm `revision`. 

This keeps your deployments reproducible. However, it can be cumbersome. 

This section shows a way to compute a charm's latest revision (based on a channel and a base) automatically using an external HTTP provider and the Charmhub API.

First, place this in `modules/charmhub/main.tf`:

```terraform
terraform {
  required_providers {
    http = {
      source  = "hashicorp/http"
      version = "~> 3.0"
    }
  }
}

variable "charm" {
  description = "Name of the charm (e.g., postgresql)"
  type        = string
}

variable "channel" {
  description = "Channel name (e.g., 14/stable, 16/edge)"
  type        = string
}

variable "base" {
  description = "Base Ubuntu (e.g., ubuntu@22.04, ubuntu@24.04)"
  type        = string
}

variable "architecture" {
  description = "Architecture (e.g., amd64, arm64)"
  type        = string
  default     = "amd64"
}

data "http" "charmhub_info" {
  url = "https://api.charmhub.io/v2/charms/info/${var.charm}?fields=channel-map.revision.revision"

  request_headers = {
    Accept = "application/json"
  }

  lifecycle {
    postcondition {
      condition     = self.status_code == 200
      error_message = "Failed to fetch charm info from Charmhub API"
    }
  }
}

locals {
  charmhub_response = jsondecode(data.http.charmhub_info.response_body)
  base_version      = split("@", var.base)[1]
  
  matching_channels = [
    for entry in local.charmhub_response["channel-map"] :
    entry if(
      entry.channel.name == var.channel &&
      entry.channel.base.channel == local.base_version &&
      entry.channel.base.architecture == var.architecture
    )
  ]

  revision = length(local.matching_channels) > 0 ? local.matching_channels[0].revision.revision : null
}

check "revision_found" {
  assert {
    condition     = local.revision != null
    error_message = "No matching revision found for charm '${var.charm}' with channel '${var.channel}', base '${var.base}', and architecture '${var.architecture}'. Please verify the combination exists in Charmhub."
  }
}

output "charm_revision" {
  description = "The revision number for the specified charm channel and base"
  value       = local.revision
}
```

Then, use the module in your Terraform plan:

```terraform
terraform {
  required_providers {
    juju = {
      source = "juju/juju"
    }
    http = {
      source  = "hashicorp/http"
      version = "~> 3.0"
    }
  }
}

locals {
  channel = "2/edge"
  base = "ubuntu@24.04"
}

module "charmhub" {
  source = "./modules/charmhub"

  charm        = "alertmanager-k8s"
  channel      = local.channel
  base      = local.base
  architecture = "amd64"
}

resource "juju_application" "alertmanager" {
  model_uuid = juju_model.development.uuid
  trust      = true

  charm {
    name     = "alertmanager-k8s"
    channel  = local.channel
    revision = module.charmhub.charm_revision
    base     = local.base
  }
}
```

For deployments with multiple charms, use `for_each` to query revisions efficiently:

```terraform
locals {
  channel = "2/edge"
  base = "ubuntu@24.04"
  
  charms = {
    alertmanager = "alertmanager-k8s"
    prometheus   = "prometheus-k8s"
    grafana      = "grafana-k8s"
  }
}

module "charmhubs" {
  source   = "./modules/charmhub"
  for_each = local.charms

  charm        = each.value
  channel      = local.channel
  base      = local.base
  architecture = "amd64"
}

resource "juju_application" "apps" {
  for_each = local.charms
  
  model_uuid = juju_model.development.uuid

  charm {
    name     = each.value
    channel  = local.channel
    revision = module.charmhubs[each.key].charm_revision
    base     = local.base
  }
}
```

This works as follows:

- The module queries the Charmhub API for the specified charm's channel map.
- It filters the results to match the requested channel, base, and architecture.
- The latest revision matching these criteria is returned as output.
- The `juju_application` resource uses this revision, ensuring reproducible deployments.
- When you change the `channel` or `base` variables and run `terraform apply`, the module fetches the new latest revision, and Terraform refreshes the charm.

(update-a-charm)=
## Update a charm

To update a charm, in the application's resource definition, in the charm attribute, use a sub-attribute specifying a different revision or channel. For example:

```terraform
resource "juju_application" "this" {
  model_uuid = juju_model.development.uuid

  charm {
    name = "hello-kubecon"
    revision = 19
  }

}
```

The Terraform provider does not support refreshing the charm when the revision is not specified. When unset, the revision number is determined during application creation. If you wish to keep the revision unset, you can refresh the application manually using the `juju` CLI. However, note that setting both `channel` and `revision` makes for a more reproducible deployment.

When the charm is changed, its resources will also be updated unless pinned.

> **Tip:** You can also use an external HTTP provider to automatically fetch the latest revision for a given channel. See {ref}`compute-a-charms-revision-automatically`.
> 
> See more: [`juju_application` > `charm` > nested schema ](../reference/terraform-provider/resources/application)

## Remove a charm

As a charm is just the *means* by which (an) application(s) are deployed, there is no way to remove the *charm* / *bundle*. What you *can* do, however, is remove the *application* / *model*.

> See more: {ref}`remove-an-application`, {ref}`destroy-a-model`
