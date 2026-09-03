---
myst:
  html_meta:
    description: "Learn how to import a manually deployed Juju model into a Terraform plan using terraform query and an AI agent with the model export fixup skill."
---

(import-a-manually-created-model)=
# Import a manually deployed model into a Terraform plan

If you have a Juju model that was created outside Terraform (e.g. with the Juju CLI) and you want to bring it under Terraform management, `terraform query` can generate the configuration, and an AI agent using the [model export fixup](https://github.com/juju/terraform-provider-juju/blob/main/docs-rtd/howto/skills/tf-model-export-fixup.md) skill can refine it into a clean, maintainable plan.

This is a subset of what `terraform query` can do. See {ref}`manage-models` for the full export workflow and other model management tasks.

## 1. Get a query file

Start from the [example query file](https://github.com/juju/terraform-provider-juju/blob/main/examples/list-resources/export-all-model.tfquery.hcl) in the provider repository. Save it as `export-all-model.tfquery.hcl` in your working directory. It exports every supported resource type from a single model.

You can adjust the query file to suit your needs — for example, remove `list` blocks for resource types you don't want to export, or add filters (e.g. `name` on `juju_space`) to narrow the results.

## 2. Ensure the provider can reach the controller

The provider needs the controller address and credentials. The simplest way is to use the Juju CLI's stored credentials — the provider reads them automatically when no explicit provider block is set.

> **Tip**: It may be useful to set up Terraform with a read-only Juju user's credentials to ensure no changes can be accidentally made to the model during the process. See [Juju | User access levels](https://documentation.ubuntu.com/juju/latest/reference/user/#valid-access-levels-for-models).

> See more: {ref}`set-up-the-terraform-provider-for-juju`

## 3. Generate the configuration

```bash
TF_VAR_model_uuid="<model-uuid>" terraform query --generate-config-out=exported.tf
```

> **Tip**: Get the model UUID with `juju show-model <model-name> --format yaml | grep model-uuid` or `juju models --format yaml`.

## 4. Set up the exported config for `terraform plan`

The generated file has no `terraform`/`required_providers` block. Place it in a sibling `versions.tf`:

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

Then run `terraform init` to install the provider.

## 5. Load the model export fixup skill

The [model export fixup skill](https://github.com/juju/terraform-provider-juju/blob/main/docs-rtd/howto/skills/tf-model-export-fixup.md) guides an AI agent through the whole refinement: rewriting literal UUIDs and names into cross-resource references, pruning attributes that can't be set in config, removing unmanageable resources, resolving drift, handling controller-set defaults, and splitting the result into maintainable files.

Download the skill file (or point your agent at the URL) and invoke it on `exported.tf`.

## 6. Iterate with the agent

The agent will run `terraform plan`, classify any errors or drift, and apply fixes — asking you to confirm judgment calls (e.g. whether to keep controller-set config defaults, whether machines should be implicit). It may ask you to run `terraform plan` or `juju status` and paste the output if it can't reach the controller itself.

Note that just because the skill tries to constrain its behaviour doesn't mean it's impossible for it to do something dangerous. Always review what commands it's trying to run.


## 7. Decide when it's good enough

The goal is a plan with no unexpected changes, but some spurious diffs (e.g. config defaults) may remain. When you are satisfied, review the final config carefully before applying — the process is best-effort and complex models may need manual adjustments. The agent will not run `terraform apply`; that's your call.