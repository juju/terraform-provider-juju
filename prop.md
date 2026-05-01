# Export the Terraform configuration from a running Juju model.

We've all been in this situation, we've experimented a bit deploying with the Juju CLI, up until it finally works as expected,
and one of our colleagues asks "How do you get there?".

We'd love to look professional, we don't send `.sh` files around. A good Terraform plan is the way to go.

So, we start to reverse engineer our Juju deployment to get a Terraform plan, and it can be painful.

That's why the Juju Terraform provider team is happy to announce a new feature: 

Export the Terraform configuration from a running Juju model, and here we outline the steps.


## Requirement

- Terraform CLI: https://snapcraft.io/terraform
  > This feature is not supported (yet) by Opentofu. See https://github.com/opentofu/opentofu/issues/3787 
- The model in a reachable Juju controller

If you don't have a model, and you'd like to test this feature, create a model in a Juju controller with this script:

`create_model.sh`
```
#!/bin/bash
set -euo pipefail

MODEL_NAME="${1:-test4}"
SSH_KEY_FILE="/tmp/tf-list-query-test-key"

juju add-model "${MODEL_NAME}"

MODEL_UUID=$(juju show-model "${MODEL_NAME}" --format json | jq -r --arg m "${MODEL_NAME}" '.[$m]["model-uuid"]')
echo "${MODEL_UUID}"

juju deploy juju-qa-dummy-source dummy-source --model "${MODEL_NAME}"
juju deploy juju-qa-dummy-sink dummy-sink --model "${MODEL_NAME}"
juju integrate --model "${MODEL_NAME}" dummy-source dummy-sink
juju offer "${MODEL_NAME}.dummy-source:sink"
juju add-machine --model "${MODEL_NAME}"

if [[ ! -f "${SSH_KEY_FILE}" ]]; then
    ssh-keygen -t rsa -b 2048 -f "${SSH_KEY_FILE}" -N "" -C "test@tf-list-query"
fi
juju add-ssh-key --model "${MODEL_NAME}" "$(cat "${SSH_KEY_FILE}.pub")"
```

## Step 1: make sure you can reach the controller

Create your `main.tf` file.

```terraform
terraform {
  required_providers {
    juju = {
      source = "juju/juju"
    }
  }
}

provider "juju" {
  controller_addresses = "<addr>"
  username             = "<username>"
  password             = "<password>"
  ca_certificate       = "<ca_cert>
}
```

Run `terraform init && terraform plan`, you should see no errors and "No changes to infrastructure".

## Step 2: define what you'd like to export

For example, create example.tfquery.hcl with:

- model
- applications
- machines
- ssh keys
- storage pools
- integrations
- offers

```terraform
variable "model_uuid" {
  description = "UUID of the model to export"
  type        = string
}


# ---------------------------------------------------------------------------
# List the model
# ---------------------------------------------------------------------------
list "juju_model" "model" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

# ---------------------------------------------------------------------------
# List all applications in the test model
# ---------------------------------------------------------------------------
list "juju_application" "all_apps" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}


# ---------------------------------------------------------------------------
# List all offers in the test model
# ---------------------------------------------------------------------------
list "juju_offer" "all_offers" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

# ---------------------------------------------------------------------------
# List all machines in the test model
# ---------------------------------------------------------------------------
list "juju_machine" "all_machines" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

# ---------------------------------------------------------------------------
# List all SSH keys in the test model
# ---------------------------------------------------------------------------
list "juju_ssh_key" "all_ssh_keys" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

# ---------------------------------------------------------------------------
# List all integrations in the test model
# ---------------------------------------------------------------------------
list "juju_integration" "all_integrations" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

# ---------------------------------------------------------------------------
# List all secrets in the test model
# ---------------------------------------------------------------------------
list "juju_secret" "all_secrets" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}

# ---------------------------------------------------------------------------
# List all storage pools in the test model
# ---------------------------------------------------------------------------
list "juju_storage_pool" "all_storage_pools" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = var.model_uuid
  }
}
```

## Step 3: export your model

Run:
`TF_VAR_model_uuid="<model-uuid>" terraform query --generate-config-out=test.tf`

It should generate a file called `test.tf`.

An example of the generated config:
```terraform
resource "juju_application" "all_apps_0" {
  provider             = juju
  config               = null
  constraints          = "arch=amd64"
  endpoint_bindings    = null
  machines             = ["1"]
  model_uuid           = "<model-uuid>"
  name                 = "<app-name>"
  registry_credentials = null
  resources            = null
  storage_directives   = {}
  trust                = false
  charm {
    base     = "ubuntu@20.04"
    channel  = "latest/stable"
    name     = "<charm>"
    revision = <revision>
  }
}

import {
  to       = juju_application.all_apps_0
  provider = juju
  identity = {
    id = "<model-uuid>:<app-name>"
  }
}
```

## Step 4: consolidate the plan

Right now you have a plan meant to import resources, in fact if you run `terraform apply` you should see something like:

```
Plan: 19 to import, 0 to add, 6 to change, 0 to destroy.
...
...
...
Apply complete! Resources: 19 imported, 0 added, 6 changed, 0 destroyed.
```

And you can manage your resources via Terraform.

However the generated plan is not production-ready because it doesn't cross-reference references.

An example:
```terraform
resource "juju_model" "model_0" {
  provider    = juju
  ...
}

resource "juju_application" "all_apps_0" {
  ...
  # the actual model uuid, not the reference juju_model.model_0.uuid
  model_uuid           = "c1cecf1e-fe66-4589-8585-e579edd6f58b" <- 
  ...
}
```
This is a problem because if we were to use the plan like this it wouldn't create the right
dependency graph, causing problems when an update/destroy is issued.


## Next steps

We would like to know what you think about this feature, and if you'd like us to build external tooling to improve the
generated plan by modifying it to populate cross-references.




