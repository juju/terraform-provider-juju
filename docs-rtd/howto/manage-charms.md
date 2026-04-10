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

This section shows how to compute a charm's latest revision (based on a channel and a base) automatically using the built-in `juju_charm` data source.

The way it works is:
- The `juju_charm` data source queries Charmhub for the specified charm, channel, and base.
- The resolved revision is available as `data.juju_charm.<name>.revision`.
- The `juju_application` resource references this revision, ensuring reproducible deployments.
- When you change the `channel` or `base` variables and run `terraform apply`, the data source fetches the new latest revision, and Terraform refreshes the charm.

For example:

```terraform
locals {
  channel = "2/edge"
  base    = "ubuntu@24.04"
}

data "juju_charm" "alertmanager" {
  charm   = "alertmanager-k8s"
  channel = local.channel
  base    = local.base
}

resource "juju_application" "alertmanager" {
  model_uuid = juju_model.development.uuid
  trust      = true

  charm {
    name     = "alertmanager-k8s"
    channel  = local.channel
    revision = data.juju_charm.alertmanager.revision
    base     = local.base
  }
}
```

For deployments with multiple charms, use `for_each` to query revisions efficiently:

```terraform
locals {
  channel = "2/edge"
  base    = "ubuntu@24.04"

  charms = {
    alertmanager = "alertmanager-k8s"
    prometheus   = "prometheus-k8s"
    grafana      = "grafana-k8s"
  }
}

data "juju_charm" "charms" {
  for_each = local.charms

  charm   = each.value
  channel = local.channel
  base    = local.base
}

resource "juju_application" "apps" {
  for_each = local.charms

  model_uuid = juju_model.development.uuid

  charm {
    name     = each.value
    channel  = local.channel
    revision = data.juju_charm.charms[each.key].revision
    base     = local.base
  }
}
```

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

> **Tip:** You can also use the `juju_charm` data source to automatically fetch the latest revision for a given channel. See {ref}`compute-a-charms-revision-automatically`.
>
> See more: [`juju_application` > `charm` > nested schema ](../reference/terraform-provider/resources/application)

(update-a-charm-when-an-relation-would-break)=
### Update a charm when a relation would break

When a charm's relation interface changes between revisions, updating the application while an existing relation is in place causes an error similar to:

```
cannot upgrade application "<consumer>" to charm "ch:amd64/<consumer>-<revision>":
would break relation "<consumer>:<relation> <offerer>:<relation>"
```

This happens because Juju refuses to update an application when doing so would invalidate a live relation.

The solution is to use the `juju_charm` data source to track the relation's interface name, store it in a `terraform_data` resource, and attach a `replace_triggered_by` lifecycle to the `juju_integration`. When the interface name changes, Terraform will destroy the relation, update the application, and recreate the relation — in the correct order.

```terraform
locals {
  channel = "dev/edge"
}

data "juju_charm" "grafana_info" {
  charm   = "grafana-k8s"
  channel = local.channel
  base    = "ubuntu@24.04"
}

resource "juju_application" "grafana" {
  model_uuid = juju_model.development.uuid
  trust      = true

  charm {
    name     = "grafana-k8s"
    channel  = local.channel
    revision = data.juju_charm.grafana_info.revision
  }
}

resource "juju_application" "traefik" {
  model_uuid = juju_model.development.uuid
  trust      = true

  charm {
    name    = "traefik-k8s"
    channel = "latest/stable"
  }
}

resource "terraform_data" "interface" {
  input = data.juju_charm.grafana_info.requires["ingress"]
}

resource "juju_integration" "ingress" {
  model_uuid = juju_model.development.uuid

  application {
    name = juju_application.traefik.name
  }

  application {
    name     = juju_application.grafana.name
    endpoint = "ingress"
  }

  lifecycle {
    replace_triggered_by = [
      terraform_data.interface
    ]
  }
}
```

This works as follows:

- The `juju_charm` data source fetches the current interface name for the `ingress` endpoint from Charmhub.
- The `terraform_data` resource stores that interface name. When the channel or revision changes and the interface name changes with it, `terraform_data.interface` is updated.
- The `replace_triggered_by` lifecycle on `juju_integration` detects the change to `terraform_data.interface` and triggers a replacement of the integration resource.
- Terraform destroys the integration first, then updates the application, then recreates the integration with the correct endpoint — avoiding the "would break relation" error.

## Remove a charm

As a charm is just the *means* by which (an) application(s) are deployed, there is no way to remove the *charm* / *bundle*. What you *can* do, however, is remove the *application* / *model*.

> See more: {ref}`remove-an-application`, {ref}`destroy-a-model`
