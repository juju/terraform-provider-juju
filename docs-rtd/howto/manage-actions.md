---
myst:
  html_meta:
    description: "Learn how to run Juju actions on application units and consume their output using the Terraform Provider for Juju."
---

(manage-actions)=
# Manage actions

> See also: {external+juju:ref}`Juju | Action <action>`

Many charms rely on Juju actions to complete their setup or to expose operational procedures (e.g. `show-proxied-endpoints`, `pre-refresh-check`, `resume-refresh`). The Terraform Provider for Juju lets you run an action as part of a Terraform plan and use its output to drive other resources.

```{note}

The action is run and its result awaited during the resource's creation. The action's output is set as a computed field that can be used by other resources after the resource has been created.
```


## Run an action

To run an action, in your Terraform plan create a resource of the `juju_action` type, specifying the model, the application to run the action on, the name of the action, and the unit to target. For example:

```terraform
resource "juju_action" "show_proxied_endpoints" {
  model_uuid       = juju_model.development.uuid
  application_name = juju_application.traefik.name
  action_name      = "show-proxied-endpoints"
  unit             = "traefik/0"
}
```

The `unit` attribute targets a specific unit by name (e.g. `traefik/0`) or the leader unit (e.g. `traefik/leader`).

> See more: [`juju_action` (resource)](../reference/terraform-provider/resources/action)

## Pass arguments to an action

Some actions accept arguments. To pass them, in the `juju_action` resource definition add an `args` map with the `key=value` pairs the action expects. For example:

```terraform
resource "juju_action" "backup" {
  model_uuid       = juju_model.development.uuid
  application_name = juju_application.postgresql.name
  action_name      = "backup"
  unit             = "postgresql/leader"

  args = {
    backup_dir = "/var/backups/postgresql"
  }
}
```

## Use an action's output

The action's result is exposed as the `output` computed attribute, a JSON string. Use `jsondecode()` to extract values from it and feed them into other resources or locals. For example:

```terraform
resource "juju_action" "show_proxied_endpoints" {
  model_uuid       = juju_model.development.uuid
  application_name = juju_application.traefik.name
  action_name      = "show-proxied-endpoints"
  unit             = "traefik/leader"
}

locals {
  application_proxied_url = jsondecode(jsondecode(juju_action.show_proxied_endpoints.output)["proxied-endpoints"])["traefik"].url
}
```

The `output` attribute is always a JSON string, so the first `jsondecode()` turns it into a Terraform map you can index into. Some charms, however, return a value that is *itself* a JSON string — for example, the traefik charm sets the `proxied-endpoints` key to a JSON-encoded string rather than a nested object. In that case a second `jsondecode()` is needed to parse the charm's value into a map. Most charms return plain maps, so a single `jsondecode()` is enough; check your charm's action documentation to find out which shape it returns.

## Re-run an action

A `juju_action` resource only runs when it is created. Once the action has completed and its result is stored in the Terraform state, subsequent `terraform apply` runs that don't change the resource are a no-op — the action is **not** re-executed. This is by design: Juju actions are one-shot operations, and the resource represents the result of a single execution.

To force the action to run again, you must replace the resource. Because every attribute of `juju_action` uses `RequiresReplace`, any change to the config causes Terraform to destroy and recreate the resource — that is, re-run the action.

## Run an action on every apply

To run the action on every apply, use a `terraform_data` resource whose input is `timestamp()` and drive the action's replacement from it via the `replace_triggered_by` lifecycle directive:

```terraform
resource "terraform_data" "replacement" {
  input = timestamp()
}

resource "juju_action" "action" {
  model_uuid       = juju_model.development.uuid
  application_name = juju_application.postgresql.name
  action_name      = "backup"
  unit             = "postgresql/leader"

  args = {
    backup_dir = "/var/backups/postgresql"
  }

  lifecycle {
    replace_triggered_by = [
      terraform_data.replacement
    ]
  }
}
```

Because `timestamp()` evaluates to a new value on every apply, `terraform_data.replacement` changes every run, which triggers a replacement of the `juju_action` resource — re-running the action on every `terraform apply`.

```{caution}

Only use this pattern when the action is genuinely idempotent or when re-running it is safe. Re-running an action that mutates state (e.g. a backup that overwrites the previous one) may have side effects.
```

## Order an action relative to other resources

Terraform infers an implicit dependency when a `juju_action` references another resource, so the action is always run **after** the application it targets has been created. For example, the `application_name` reference below makes Terraform wait for `juju_application.traefik` to exist before enqueuing the action:

```terraform
resource "juju_action" "show_proxied_endpoints" {
  model_uuid       = juju_model.development.uuid
  application_name = juju_application.traefik.name # implicit dependency
  action_name      = "show-proxied-endpoints"
  unit             = "traefik/0"
}
```

However, Terraform cannot infer dependencies that aren't expressed through references. If the action must run only after some other condition is met — for example, after an integration has been created, after a config has been applied, or after another action has completed — add an explicit `depends_on`:

```terraform
resource "juju_integration" "db" {
  model_uuid = juju_model.development.uuid

  application {
    name     = juju_application.postgresql.name
    endpoint = "db"
  }

  application {
    name     = juju_application.myapp.name
    endpoint = "db"
  }
}

resource "juju_action" "migrate" {
  model_uuid       = juju_model.development.uuid
  application_name = juju_application.myapp.name
  action_name      = "migrate"
  unit             = "myapp/leader"

  # Run the migration only after the database integration is in place.
  depends_on = [juju_integration.db]
}
```



## Remove an action

To remove an action, remove its resource definition from your Terraform plan.

> See more: [`juju_action` (resource)](../reference/terraform-provider/resources/action)
