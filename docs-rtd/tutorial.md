---
myst:
  html_meta:
    description: "Learn to manage Juju deployments as code with the Terraform Provider, using declarative configuration files, version control, and infrastructure-as-code workflows."
---

# Manage your first Juju deployment as code

The Terraform Provider for Juju brings infrastructure-as-code capabilities to Juju. With it, you can define your cloud infrastructure and applications in version-controlled configuration files, preview changes before applying them, and collaborate with your team using familiar GitOps workflows.

In this tutorial you will define and deploy a chat service (Mattermost backed by PostgreSQL) as code, experiencing the benefits of declarative infrastructure management.

**What you'll need:**

- A workstation, e.g., a laptop, that has sufficient resources to launch a virtual machine with 4 CPUs, 8 GB RAM, and 50 GB disk space.
- Familiarity with a terminal.
- Basic familiarity with Juju concepts (controllers, models, charms, applications).
- Basic familiarity with Terraform (providers, resources, state).

**What you'll do:**

- Set up an isolated test environment with Multipass, then set up Terraform with the Juju provider to bootstrap a controller and deploy applications on MicroK8s.
- Define your infrastructure as code, preview changes before applying them, and experience infrastructure-as-code workflows.

## Set up an isolated test environment

```{important}

**Tempted to skip this step?** We strongly recommend that you do not! As you will see in a minute, the VM you set up in this step does not just provide you with an isolated test environment but also with almost everything else you’ll need in the rest of this tutorial (and the non-VM alternative may not yield exactly the same results).
```

When you're trying things out it's nice to work in an isolated test environment. Let's spin up an Ubuntu virtual machine (VM) with Multipass!

First, [install Multipass](https://documentation.ubuntu.com/multipass/en/latest/how-to-guides/install-multipass/). For example, on Linux with `snapd`:

```{terminal}
:copy:
:user:
:host:
sudo snap install multipass
```

```{important}
If on Windows: Note that Multipass can only be installed on Windows 10 Pro or Enterprise. If you are using a different version, you'll need to manually set up MicroK8s and the `juju` CLI outside of a Multipass VM.
```

Now, use Multipass to launch a Juju-ready VM using the `charm-dev` cloud-init configuration:

```{note}
This step may take a few minutes to complete (e.g., 10 mins).

This is because the command downloads, installs, (updates,) and configures a number of packages (including MicroK8s, the `juju` CLI, Terraform, and development tools), and the speed will be affected by network bandwidth.

However, once it's done, you'll have everything you'll need -- all in a nice isolated environment that you can clean up easily.
```

```{terminal}
:copy:
:user:
:host:
multipass launch 24.04 \
  --name my-juju-vm \
  --cpus 4 \
  --memory 8G \
  --disk 50G \
  --timeout 1800 \
  --cloud-init https://raw.githubusercontent.com/canonical/multipass/refs/heads/main/data/cloud-init-yaml/cloud-init-charm-dev.yaml
```

```{dropdown} Tips for troubleshooting
If the VM launch fails, run `multipass delete --purge my-juju-vm` to clean up, then try the launch command again.
```

Open a shell into the VM:

```{terminal}
:copy:
:user:
:host:
multipass shell my-juju-vm
```

Anything you type after the VM shell prompt (`ubuntu@my-juju-vm:~$`) will run on the VM.

```{dropdown} Tips for usage

At any point:
- To exit the shell, press {kbd}`Ctrl` + {kbd}`D` or type `exit`.
- To stop the VM after exiting the VM shell, run `multipass stop my-juju-vm`.
- To restart the VM and re-open a shell into it, type `multipass shell my-juju-vm`.
```

Congratulations! Your cloud is ready, and thanks to the `charm-dev` cloud-init, you already have:
- MicroK8s configured and running
- The `juju` CLI installed
- Terraform installed
- A MicroK8s cloud registered with Juju

On your local workstation (not the VM), create a directory for your Terraform configuration and mount it to your VM:

```{terminal}
:copy:
:user:
:host:
mkdir ~/terraform-juju && cd ~/terraform-juju
multipass mount ~/terraform-juju my-juju-vm:~/terraform-juju
```

This setup will enable you to create and edit Terraform files in your local editor while running them inside your VM.

## Set up version control

A key benefit of infrastructure-as-code is version control. Your infrastructure definitions become code that can be tracked, reviewed, and collaborated on.

On your local workstation, in your `terraform-juju` directory, initialize a git repository:

```{terminal}
:copy:
:user:
:host:
git init
```

As you create and modify files in this tutorial, you'll commit them to track your infrastructure's evolution.

## Set up Terraform with the Juju provider

The way Terraform with the Juju provider works is: you define your desired infrastructure state in `.tf` files, the `terraform` CLI reads those files and talks to a Juju controller via the `juju` provider plugin, and the controller provisions resources and deploys applications. Your infrastructure definitions are code that can be version-controlled, reviewed, and shared with your team.

Thanks to the `charm-dev` cloud-init, the `terraform` CLI is already installed in your VM. You can verify this:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform version
```

## Bootstrap a Juju controller

A Juju controller is your Juju control plane -- the entity that holds the Juju API server and Juju's database. With the Terraform Provider, you can bootstrap a controller declaratively by defining it in your Terraform configuration.

Thanks to the `charm-dev` cloud-init, the `juju` CLI is already installed and the MicroK8s cloud is already registered with Juju. You can verify this in your VM:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
juju clouds --client
```

You should see `microk8s` listed. Now, view the MicroK8s credentials that you'll need for your Terraform configuration:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
juju show-credentials microk8s --show-secrets --client
```

From the output, note the values for the client certificate, client key, server certificate, and endpoint. You'll use these in your Terraform configuration.

Now, on your local workstation, in your `terraform-juju` directory, create your Terraform configuration files.

First, create `terraform.tf` to configure Terraform to use the Juju provider in controller mode:

```{code-block} terraform
:caption: `terraform.tf`

terraform {
  required_providers {
    juju = {
      version = "~> 1.0.0"
      source  = "juju/juju"
    }
  }
}

provider "juju" {
  controller_mode = true
}
```

Next, create `variables.tf` to define variables for your sensitive credentials:

```{code-block} terraform
:caption: `variables.tf`

variable "k8s_endpoint" {
  description = "MicroK8s API endpoint"
  type        = string
}

variable "k8s_ca_cert" {
  description = "MicroK8s CA certificate"
  type        = string
  sensitive   = true
}

variable "k8s_client_cert" {
  description = "MicroK8s client certificate"
  type        = string
  sensitive   = true
}

variable "k8s_client_key" {
  description = "MicroK8s client key"
  type        = string
  sensitive   = true
}
```

Create `terraform.tfvars` with your actual credential values (from the `juju show-credentials` output):

```{code-block} terraform
:caption: `terraform.tfvars`

k8s_endpoint    = "https://<your-microk8s-ip>:16443"
k8s_ca_cert     = "<your-ca-certificate>"
k8s_client_cert = "<your-client-certificate>"
k8s_client_key  = "<your-client-key>"
```

```{important}
**Keep credentials out of version control!** Add `terraform.tfvars` to your `.gitignore` file:

```{terminal}
:copy:
:user:
:host:
echo "terraform.tfvars" >> .gitignore
echo ".terraform*" >> .gitignore
echo "terraform.tfstate*" >> .gitignore
```
```

Now, create `main.tf` to define your controller:

```{code-block} terraform
:caption: `main.tf`

resource "juju_controller" "microk8s" {
  name = "my-chat-controller"

  # Use the snap-installed juju binary to avoid confinement issues
  juju_binary = "/snap/juju/current/bin/juju"

  cloud = {
    name       = "microk8s"
    type       = "kubernetes"
    auth_types = ["clientcertificate"]
    endpoint   = var.k8s_endpoint
    ca_certificates = [var.k8s_ca_cert]
    host_cloud_region = "localhost"
  }

  cloud_credential = {
    name      = "microk8s-cred"
    auth_type = "clientcertificate"
    attributes = {
      "ClientCertificateData" = var.k8s_client_cert
      "ClientKeyData"         = var.k8s_client_key
    }
  }

  lifecycle {
    ignore_changes = [
      cloud.region,
      cloud.host_cloud_region
    ]
  }
}
```

Notice how this declarative definition makes your infrastructure intentions clear: you want a Juju controller named "my-chat-controller" on MicroK8s with specific credentials.

Commit your infrastructure definition (excluding sensitive files):

```{terminal}
:copy:
:user:
:host:
git add terraform.tf variables.tf main.tf .gitignore
git commit -m "feat: define Juju controller infrastructure"
```

> **Infrastructure-as-code benefit**: Your controller infrastructure is now tracked in version control. You can see the history of changes, revert if needed, and share with your team for review.

Now, in your VM, initialize Terraform and preview your changes:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
cd ~/terraform-juju
terraform init
```

This downloads the Juju provider plugin and prepares your workspace.

Preview what Terraform will create:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform plan
```

This shows what Terraform will create without actually creating anything. Review the output carefully -- this is your opportunity to catch issues before they affect your infrastructure.

> **Infrastructure-as-code benefit**: The plan step lets you (and your team) review changes before applying them. In a team setting, you'd commit your `.tf` changes, open a pull request, and have teammates review the plan output before merging and applying.

Apply your infrastructure definition:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform apply
```

Terraform will show you the plan again and ask for confirmation. Type `yes` to proceed.

The bootstrap process will take a few minutes. Once complete, your Juju controller is running on MicroK8s, and Terraform has recorded its state.

Congratulations! You've bootstrapped a Juju controller as code.


## Define your application infrastructure as code

With your controller bootstrapped, now define the applications that make up your chat service. You'll deploy Mattermost for the chat service, PostgreSQL for its backing database, and self-signed certificates to TLS-encrypt traffic from PostgreSQL.

First, update your Terraform configuration to switch from controller mode to regular mode. On your local workstation, in `terraform.tf`, remove the `controller_mode` setting and configure the provider to use your bootstrapped controller:

```{code-block} terraform
:caption: `terraform.tf`

terraform {
  required_providers {
    juju = {
      version = "~> 1.0.0"
      source  = "juju/juju"
    }
  }
}

provider "juju" {
  # The provider will automatically use the controller bootstrapped by Terraform
  # by reading from the terraform state
}
```

Now, update `main.tf` to add your application resources. Replace the entire contents with:

```{code-block} terraform
:caption: `main.tf`

resource "juju_controller" "microk8s" {
  name = "my-chat-controller"

  juju_binary = "/snap/juju/current/bin/juju"

  cloud = {
    name       = "microk8s"
    type       = "kubernetes"
    auth_types = ["clientcertificate"]
    endpoint   = var.k8s_endpoint
    ca_certificates = [var.k8s_ca_cert]
    host_cloud_region = "localhost"
  }

  cloud_credential = {
    name      = "microk8s-cred"
    auth_type = "clientcertificate"
    attributes = {
      "ClientCertificateData" = var.k8s_client_cert
      "ClientKeyData"         = var.k8s_client_key
    }
  }

  lifecycle {
    ignore_changes = [
      cloud.region,
      cloud.host_cloud_region
    ]
  }
}

# Define the workspace for your applications
resource "juju_model" "chat" {
  name = "chat"

  # Ensure the controller is bootstrapped first
  depends_on = [juju_controller.microk8s]
}

# Define the chat application
resource "juju_application" "mattermost-k8s" {
  model = juju_model.chat.name

  charm {
    name = "mattermost-k8s"
  }
}

# Define the database with high availability
resource "juju_application" "postgresql-k8s" {
  model = juju_model.chat.name

  charm {
    name    = "postgresql-k8s"
    channel = "14/stable"
  }

  trust = true
  units = 2  # High availability configuration

  config = {
    profile = "testing"
  }
}

# Define the TLS certificate provider
resource "juju_application" "self-signed-certificates" {
  model = juju_model.chat.name

  charm {
    name = "self-signed-certificates"
  }
}

# Integrate PostgreSQL with Mattermost
resource "juju_integration" "postgresql-mattermost" {
  model = juju_model.chat.name

  application {
    name     = juju_application.postgresql-k8s.name
    endpoint = "db"
  }

  application {
    name = juju_application.mattermost-k8s.name
  }

  lifecycle {
    replace_triggered_by = [
      juju_application.postgresql-k8s.name,
      juju_application.postgresql-k8s.model,
      juju_application.postgresql-k8s.constraints,
      juju_application.postgresql-k8s.placement,
      juju_application.postgresql-k8s.charm.name,
      juju_application.mattermost-k8s.name,
      juju_application.mattermost-k8s.model,
      juju_application.mattermost-k8s.constraints,
      juju_application.mattermost-k8s.placement,
      juju_application.mattermost-k8s.charm.name,
    ]
  }
}

# Integrate PostgreSQL with TLS certificates
resource "juju_integration" "postgresql-tls" {
  model = juju_model.chat.name

  application {
    name = juju_application.postgresql-k8s.name
  }

  application {
    name = juju_application.self-signed-certificates.name
  }

  lifecycle {
    replace_triggered_by = [
      juju_application.postgresql-k8s.name,
      juju_application.postgresql-k8s.model,
      juju_application.postgresql-k8s.constraints,
      juju_application.postgresql-k8s.placement,
      juju_application.postgresql-k8s.charm.name,
      juju_application.self-signed-certificates.name,
      juju_application.self-signed-certificates.model,
      juju_application.self-signed-certificates.constraints,
      juju_application.self-signed-certificates.placement,
      juju_application.self-signed-certificates.charm.name,
    ]
  }
}
```

Notice how this declarative definition makes your infrastructure intentions clear: you want a chat model with Mattermost, PostgreSQL (with 2 units for high availability), TLS certificates, and specific integrations between them.

Commit your application infrastructure definition:

```{terminal}
:copy:
:user:
:host:
git add main.tf terraform.tf
git commit -m "feat: define chat application infrastructure"
```

> **Infrastructure-as-code benefit**: Your entire application infrastructure is now defined as code and tracked in version control. Anyone reviewing your git history can see exactly what applications are deployed, how they're configured, and how they integrate.

## Deploy your application infrastructure

Unlike imperative commands that execute immediately, Terraform's workflow includes a review step. You'll see what Terraform plans to do before any changes are made.

In your VM, preview the changes:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform plan
```

This shows what Terraform will create without actually creating anything. Review the output carefully -- you'll see the model, applications, and integrations that will be created.

> **Infrastructure-as-code benefit**: The plan step lets you (and your team) review changes before applying them. In a team setting, you'd commit your `.tf` changes, open a pull request, and have teammates review the plan output before merging and applying.

Apply your infrastructure definition:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform apply
```

Terraform will show you the plan again and ask for confirmation. Type `yes` to proceed.

Watch the deployment progress. You can use the `juju` CLI to inspect status:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
juju status --relations --watch 1s
```

Things are all set when your `App Status` shows `active` and your `Unit - Workload` shows `active`.

Once deployed, test your chat service. From the output of `juju status` > `Unit` > `mattermost-k8s/0`, retrieve the IP address and port, then use `curl` to test:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
curl <IP address>:<port>/api/v4/system/ping
```

Sample output:

```text
{"ActiveSearchBackend":"database","AndroidLatestVersion":"","AndroidMinVersion":"","IosLatestVersion":"","IosMinVersion":"","status":"OK"}
```

Congratulations! Your chat service is up and running, and your entire infrastructure is defined as code.

> **Infrastructure-as-code benefit**: Terraform's state tracking means you can't accidentally create duplicate resources. It knows what exists and only makes necessary changes.

## Manage your infrastructure

A key benefit of infrastructure-as-code is that the same workflow handles all changes. Let's scale your PostgreSQL database for improved availability.

On your local workstation, in `main.tf`, modify the `postgresql-k8s` resource to change `units` from `2` to `3`:

```{code-block} terraform
:caption: `main.tf`

resource "juju_application" "postgresql-k8s" {
  model = juju_model.chat.name

  charm {
    name    = "postgresql-k8s"
    channel = "14/stable"
  }

  trust = true
  units = 3  # Changed from 2

  config = {
    profile = "testing"
  }
}
```

In your VM, preview the change:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform plan
```

Notice Terraform detected the difference between your desired state (3 units) and actual state (2 units), and shows it will add one unit. This is the power of declarative infrastructure -- you describe what you want, and Terraform figures out how to get there.

> **Infrastructure-as-code benefit**: Terraform's state tracking prevents accidental changes. It knows the current state and only makes necessary modifications to reach your desired state.

Apply the change:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform apply
```

Watch the scaling operation with `juju status --relations`.

On your local workstation, commit your change:

```{terminal}
:copy:
:user:
:host:
git add main.tf
git commit -m "feat: scale postgresql to 3 units for improved availability"
```

> **Infrastructure-as-code benefit**: Your git history now shows why and when you scaled. Anyone on your team can see the evolution of your infrastructure and the reasoning behind each change (captured in commit messages).

## Next steps

You've experienced the core infrastructure-as-code workflow with the Terraform Provider for Juju. To build on what you've learned:

- **Bootstrap with more control**: Configure controller and model settings during bootstrap. See: {ref}`configure-a-controller`
- **Manage controllers post-bootstrap**: Configure controllers, enable high availability, or import existing controllers. See: {ref}`manage-controllers`
- **Manage multiple environments**: Use Terraform workspaces or separate configurations to manage dev, staging, and production. See: {ref}`manage-models`
- **Integrate with other cloud resources**: Combine the Juju provider with AWS, GCP, or Azure providers to manage applications and underlying cloud resources together in a single Terraform plan.
- **Enable team collaboration**: Set up remote state storage and implement GitOps workflows for infrastructure changes.
- **Explore all provider features**: The provider supports credentials, users, offers, secrets, and more. See: {ref}`reference`


## Tear down your test environment

With Terraform, tearing down your infrastructure is as simple as deploying it.

In your VM, destroy all Terraform-managed infrastructure:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform destroy
```

Terraform will show you everything it will remove and ask for confirmation. Type `yes` to proceed.

This removes all infrastructure Terraform created: applications, integrations, model, and controller. Notice how Terraform maintains consistency -- it knows exactly what it created from the state, so it can cleanly remove everything.

Exit the VM:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
exit
```

From your local workstation, delete the VM:

```{terminal}
:copy:
:user:
:host:
multipass delete --purge my-juju-vm
```

Finally, [uninstall Multipass](https://documentation.ubuntu.com/multipass/en/latest/how-to-guides/install-multipass/#uninstall) if you no longer need it.

Your local `terraform-juju` directory contains your infrastructure definitions -- keep this git repository to preserve your infrastructure history, or delete it if you're done experimenting.

