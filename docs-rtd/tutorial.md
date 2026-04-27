---
myst:
  html_meta:
    description: "Learn to manage Juju deployments as code with the Terraform Provider, using declarative configuration files, version control, and infrastructure-as-code workflows."
---

# Get started with the Terraform Provider for Juju

The Terraform Provider for Juju brings infrastructure-as-code capabilities to {external+juju:doc}`Juju <index>`. With it, you can define your cloud infrastructure and applications in version-controlled configuration files, preview changes before applying them, and collaborate with your team using familiar GitOps workflows.

In this tutorial you will define and deploy a chat service (Mattermost backed by PostgreSQL) as code, experiencing the benefits of declarative infrastructure management.

**What you'll need:**

- A workstation, e.g., a laptop, that has sufficient resources to launch a virtual machine with 4 CPUs, 8 GB RAM, and 50 GB disk space.
- Familiarity with a terminal.
- Basic familiarity with Juju concepts (controllers, models, charms, applications).
- Basic familiarity with Terraform (providers, resources, state).

**What you'll do:**

- Set up your environment: launch a VM with MicroK8s, install Terraform, initialize version control, and bootstrap a Juju controller.
- Define your application infrastructure as code, preview changes before applying them, and deploy a chat service.
- Experience infrastructure-as-code workflows: manage your infrastructure, track changes in version control, and tear down resources.

## Set up your environment

```{tip}
**Recommended workflow setup:**

You'll be working across two contexts: your local workstation and a VM. To work efficiently:

1. **Two terminal windows (or one split terminal):**
   - One terminal for your local workstation (where you'll run `multipass` commands and `git` commands)
   - One terminal for the VM shell (where you'll run `terraform` and `juju` commands)

2. **Your favorite text editor** on your local workstation to create and edit `.tf` files. Changes you make locally will be automatically visible in the VM via the mounted directory.
```

To work with the Terraform Provider for Juju, you'll need:
- A cloud (MicroK8s for this tutorial)
- The `juju` CLI (to extract cloud credentials and to bootstrap controllers -- the Terraform provider calls `juju` commands in the background)
- The `terraform` CLI (to run your infrastructure-as-code definitions)
- A Juju controller (which you'll bootstrap with Terraform)

You'll get most of these automatically by launching a Juju-ready Ubuntu VM with Multipass using the `charm-dev` cloud-init configuration, then install Terraform manually.

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
- A MicroK8s cloud registered with Juju

Verify this in your VM:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
microk8s status --wait-ready

microk8s is running
```

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
juju version

3.6.0-ubuntu-amd64
```

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
juju clouds --client

Cloud      Regions  Default    Type
localhost  1        localhost  lxd
microk8s   1        localhost  k8s
```

Now install Terraform in your VM:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
sudo snap install terraform --classic
```

Verify the installation:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform version
```

The Terraform Provider for Juju works by calling the `juju` CLI in the background. When you run `terraform apply`, Terraform will call `juju bootstrap`, and Juju needs MicroK8s credentials to connect to your cluster. Copy the credentials to where Juju expects to find them when called by Terraform:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
sudo sh -c "mkdir -p /var/snap/terraform/current/microk8s/credentials" && sudo sh -c "microk8s config | tee /var/snap/terraform/current/microk8s/credentials/client.config" && sudo chown -f -R $USER:$USER /var/snap/terraform/current/microk8s/credentials/client.config
```

```{dropdown} Why is this necessary?

Both Terraform and Juju are installed as snaps. When snap-installed Terraform calls snap-installed Juju, Juju looks for MicroK8s credentials in Terraform's snap storage directory (`/var/snap/terraform/current/`), not in MicroK8s's own directory.

Since MicroK8s was installed by charm-dev and is not strictly confined, its credentials aren't automatically shared with the Terraform snap. This command copies the credentials to where Terraform's Juju can find them.

This is the same credential-sharing pattern used in Juju's documentation when working with non-strictly-confined MicroK8s.
```

Now, on your local workstation (not the VM), create a directory for your Terraform configuration and mount it to your VM:

```{terminal}
:copy:
:user:
:host:
mkdir ~/terraform-juju && cd ~/terraform-juju && multipass mount ~/terraform-juju my-juju-vm:terraform-juju
```

This lets you create and edit Terraform files in your local editor while running them inside the VM.

Now set up version control. A key benefit of infrastructure-as-code is version control -- your infrastructure definitions become code that can be tracked, reviewed, and collaborated on.

On your local workstation, in your `terraform-juju` directory, initialize a git repository:

```{terminal}
:copy:
:user:
:host:
git init
```

As you create and modify files in this tutorial, you'll commit them to track your infrastructure's evolution.

Create two subdirectories to organize your infrastructure:

```{terminal}
:copy:
:user:
:host:
mkdir 1-bootstrap 2-deploy
```

```{note}
**Why two directories?**

The Terraform Juju provider has a `controller_mode` setting that determines which resources you can manage:
- When `controller_mode = true`: You can ONLY manage `juju_controller` resources (to bootstrap controllers)
- When `controller_mode = false` (or omitted): You can manage everything EXCEPT `juju_controller` resources (models, applications, integrations, etc.)

This design requires separating controller bootstrap from application deployment into two distinct Terraform configurations. This tutorial uses `1-bootstrap/` for the controller and `2-deploy/` for your applications.
```

Before bootstrapping a controller, it's helpful to understand how the Terraform Provider for Juju works. The provider is a Juju client -- it talks to a Juju controller just like the `juju` CLI does. You define your desired infrastructure state in `.tf` files, the `terraform` CLI reads those files and uses the Juju provider plugin to communicate with a Juju controller, and the controller provisions resources and deploys applications. Your infrastructure definitions are code that can be version-controlled, reviewed, and shared with your team.

```{figure}
:align: center

:::{mermaid}
graph LR
    A[.tf files<br/>Infrastructure as Code] --> B[terraform CLI]
    B --> C[Juju Provider<br/>Juju client]
    C --> D[Juju Controller]
    D --> E[Cloud<br/>MicroK8s]
    D --> F[Charmhub]

    style A fill:#e1f5ff
    style C fill:#fff3e0
    style D fill:#f3e5f5
:::

The Terraform Provider acts as a Juju client, translating your infrastructure-as-code definitions into API calls to the Juju controller. The controller then provisions resources on the cloud and deploys charms from Charmhub.
```

Now bootstrap a Juju controller. A Juju controller is your Juju control plane -- the entity that holds the Juju API server and Juju's database. With the Terraform Provider, you can bootstrap a controller by defining it in your Terraform configuration.

View the MicroK8s credentials that you'll need for your Terraform configuration:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
juju credentials microk8s --show-secrets --format yaml

client-credentials:
  microk8s:
    default-credential: microk8s
    microk8s:
      auth-type: oauth2
      Token: eyJhbGciOiJSUzI1NiIsImtpZCI6IldBbERh...
```

From the output, copy the full `Token` value (it will be much longer than shown here). You'll also need the MicroK8s endpoint and CA certificate, which you can get from the kubeconfig:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
microk8s config

apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTi...
    server: https://10.x.x.x:16443
  name: microk8s-cluster
...
```

From the output, copy the full `certificate-authority-data` value and the `server` (endpoint) URL.

## Bootstrap a controller

Now, on your local workstation, create your controller bootstrap configuration in the `1-bootstrap/` directory.

First, create `1-bootstrap/terraform.tf` to configure Terraform to use the Juju provider in controller mode:

```{code-block} terraform
:caption: `1-bootstrap/terraform.tf`

terraform {
  required_providers {
    juju = {
      version = "~> 1.4"
      source  = "juju/juju"
    }
  }
}

provider "juju" {
  controller_mode = true
}
```

```{important}
Notice `controller_mode = true`. This setting restricts the provider to only managing `juju_controller` resources. You cannot define models, applications, or integrations in this configuration.
```

Next, create `1-bootstrap/variables.tf` to define variables for your sensitive credentials:

```{code-block} terraform
:caption: `1-bootstrap/variables.tf`

variable "k8s_endpoint" {
  description = "MicroK8s API endpoint"
  type        = string
}

variable "k8s_ca_cert" {
  description = "MicroK8s CA certificate"
  type        = string
  sensitive   = true
}

variable "k8s_token" {
  description = "MicroK8s authentication token"
  type        = string
  sensitive   = true
}
```

Create `1-bootstrap/terraform.tfvars` with your actual credential values (from the commands above):

```{code-block} terraform
:caption: `1-bootstrap/terraform.tfvars`

k8s_token    = "eyJhbGciOiJSUzI1NiIsImtpZCI6IldBbERh..."
k8s_ca_cert  = "LS0tLS1CRUdJTi..."
k8s_endpoint = "https://10.x.x.x:16443"
```

```{note}
The values shown above are examples only. Use your actual values from the previous commands - the token and certificate will be much longer than shown here.
```

Before continuing, keep credentials and Terraform state out of version control. On your local workstation, create a `.gitignore` file in the `1-bootstrap/` directory:

```{terminal}
:copy:
:user:
:host:
echo "terraform.tfvars" >> 1-bootstrap/.gitignore && echo ".terraform*" >> 1-bootstrap/.gitignore && echo "terraform.tfstate*" >> 1-bootstrap/.gitignore
```

Now, create `1-bootstrap/main.tf` to define your controller:

```{code-block} terraform
:caption: `1-bootstrap/main.tf`

resource "juju_controller" "microk8s" {
  name = "my-chat-controller"

  juju_binary = "/snap/juju/current/bin/juju"

  cloud = {
    name       = "microk8s"
    type       = "kubernetes"
    auth_types = ["oauth2"]
    endpoint   = var.k8s_endpoint
    ca_certificates = [var.k8s_ca_cert]
    host_cloud_region = "localhost"
  }

  cloud_credential = {
    name      = "microk8s-cred"
    auth_type = "oauth2"
    attributes = {
      "Token" = var.k8s_token
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

Notice how this declarative definition makes your infrastructure intentions clear: you want a Juju controller named `my-chat-controller` on MicroK8s with specific credentials.

Commit your controller infrastructure definition (excluding sensitive files):

```{terminal}
:copy:
:user:
:host:
git add 1-bootstrap && git commit -m "feat: define Juju controller infrastructure"
```

```{tip}
**Infrastructure-as-code benefit**: Your controller infrastructure is now tracked in version control. You can see the history of changes, revert if needed, and share with your team for review.
```

Now, in your VM, initialize Terraform in the bootstrap directory. If you exited the VM shell, reopen it:

```{terminal}
:copy:
:user:
:host:
multipass shell my-juju-vm
```

Navigate to your bootstrap directory and initialize:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
:dir: ~/terraform-juju
cd 1-bootstrap && terraform init
```

This downloads the Juju provider plugin and prepares your workspace.

Preview what Terraform will create:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
:dir: ~/terraform-juju/1-bootstrap
terraform plan
```

This shows what Terraform will create without actually creating anything. Review the output carefully -- you'll see the controller resource that will be created.

```{tip}
**Infrastructure-as-code benefit**: The plan step lets you (and your team) review changes before applying them. In a team setting, you'd commit your `.tf` changes, open a pull request, and have teammates review the plan output before merging and applying.
```

Apply your infrastructure definition:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
:dir: ~/terraform-juju/1-bootstrap
terraform apply
```

Terraform will show you the plan again and ask for confirmation. Type `yes` to proceed.

The bootstrap process will take a few minutes. Once complete, your Juju controller is running on MicroK8s, and Terraform has recorded its state.

Your controller is now bootstrapped and ready. Next, you'll define and deploy applications.

## Define your application infrastructure as code

With your controller bootstrapped, now define the applications that make up your chat service in a separate configuration. You'll deploy Mattermost for the chat service, PostgreSQL for its backing database, and self-signed certificates to TLS-encrypt traffic from PostgreSQL.

On your local workstation, create your application deployment configuration in the `2-deploy/` directory.

First, create `2-deploy/terraform.tf` to configure the provider without `controller_mode`:

```{code-block} terraform
:caption: `2-deploy/terraform.tf`

terraform {
  required_providers {
    juju = {
      version = "~> 1.4"
      source  = "juju/juju"
    }
  }
}

provider "juju" {
  # Connect to the bootstrapped controller
  # The provider will discover the controller automatically
}
```

```{important}
Notice there's no `controller_mode` setting (it defaults to `false`). This configuration can manage models, applications, and integrations, but cannot manage `juju_controller` resources.
```

Create a `.gitignore` file for this directory:

```{terminal}
:copy:
:user:
:host:
echo ".terraform*" >> 2-deploy/.gitignore && echo "terraform.tfstate*" >> 2-deploy/.gitignore
```

Now, create `2-deploy/main.tf` to define your application resources:

```{code-block} terraform
:caption: `2-deploy/main.tf`

# Define the workspace for your applications
resource "juju_model" "chat" {
  name = "chat"
}

# Define the chat application
resource "juju_application" "mattermost-k8s" {
  model_uuid = juju_model.chat.uuid

  charm {
    name = "mattermost-k8s"
  }
}

# Define the database with high availability
resource "juju_application" "postgresql-k8s" {
  model_uuid = juju_model.chat.uuid

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
  model_uuid = juju_model.chat.uuid

  charm {
    name = "self-signed-certificates"
  }
}

# Integrate PostgreSQL with Mattermost
resource "juju_integration" "postgresql-mattermost" {
  model_uuid = juju_model.chat.uuid

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
      juju_application.postgresql-k8s.model_uuid,
      juju_application.postgresql-k8s.constraints,
      juju_application.postgresql-k8s.placement,
      juju_application.postgresql-k8s.charm.name,
      juju_application.mattermost-k8s.name,
      juju_application.mattermost-k8s.model_uuid,
      juju_application.mattermost-k8s.constraints,
      juju_application.mattermost-k8s.placement,
      juju_application.mattermost-k8s.charm.name,
    ]
  }
}

# Integrate PostgreSQL with TLS certificates
resource "juju_integration" "postgresql-tls" {
  model_uuid = juju_model.chat.uuid

  application {
    name     = juju_application.postgresql-k8s.name
    endpoint = "certificates"
  }

  application {
    name     = juju_application.self-signed-certificates.name
    endpoint = "certificates"
  }

  lifecycle {
    replace_triggered_by = [
      juju_application.postgresql-k8s.name,
      juju_application.postgresql-k8s.model_uuid,
      juju_application.postgresql-k8s.constraints,
      juju_application.postgresql-k8s.placement,
      juju_application.postgresql-k8s.charm.name,
      juju_application.self-signed-certificates.name,
      juju_application.self-signed-certificates.model_uuid,
      juju_application.self-signed-certificates.constraints,
      juju_application.self-signed-certificates.placement,
      juju_application.self-signed-certificates.charm.name,
    ]
  }
}
```

Notice how this declarative definition makes your infrastructure intentions clear: you want a chat model with Mattermost, PostgreSQL (with 2 units for high availability), TLS certificates, and specific integrations between them.

```{note}
Notice the use of `model_uuid` instead of `model` for applications. This ensures Terraform uses the UUID reference, which is more reliable than name-based references.

Also notice the explicit `endpoint` values in the TLS integration -- this ensures the correct relation endpoints are used.
```

Commit your application infrastructure definition:

```{terminal}
:copy:
:user:
:host:
git add 2-deploy && git commit -m "feat: define chat application infrastructure"
```

```{tip}
**Infrastructure-as-code benefit**: Your entire application infrastructure is now defined as code and tracked in version control. Anyone reviewing your git history can see exactly what applications are deployed, how they're configured, and how they integrate.
```

## Deploy your application infrastructure

Unlike imperative commands that execute immediately, Terraform's workflow includes a review step. You'll see what Terraform plans to do before any changes are made.

```{figure}
:align: center

:::{mermaid}
graph TB
    A[Modify .tf files] --> B[terraform plan]
    B --> C{Review changes}
    C -->|Approve| D[terraform apply]
    C -->|Revise| A
    D --> E[Update infrastructure]
    E --> F[Update state file]

    style B fill:#e3f2fd
    style C fill:#fff9c4
    style D fill:#e8f5e9
    style F fill:#fce4ec
:::

Terraform's plan-review-apply workflow ensures you always preview infrastructure changes before they're executed. This is a key infrastructure-as-code benefit for team collaboration and change management.
```

In your VM, navigate to the deployment directory, initialize, and preview the changes:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
:dir: ~/terraform-juju/1-bootstrap
cd ../2-deploy && terraform init
```

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
:dir: ~/terraform-juju/2-deploy
terraform plan
```

This shows what Terraform will create without actually creating anything. Review the output carefully -- you'll see the model, applications, and integrations that will be created.

```{tip}
**Infrastructure-as-code benefit**: The plan step lets you (and your team) review changes before applying them. In a team setting, you'd commit your `.tf` changes, open a pull request, and have teammates review the plan output before merging and applying.
```

Apply your infrastructure definition:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
:dir: ~/terraform-juju/2-deploy
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

```{figure}
:align: center

:::{mermaid}
graph TB
    subgraph "MicroK8s Cloud"
        subgraph "Controller Model"
            C[Juju Controller]
        end

        subgraph "Chat Model"
            M[Mattermost<br/>mattermost-k8s/0]
            P1[PostgreSQL<br/>postgresql-k8s/0<br/>Primary]
            P2[PostgreSQL<br/>postgresql-k8s/1]
            S[Self-signed Certs<br/>self-signed-certificates/0]

            P1 -.db relation.-> M
            S -.certificates relation.-> P1
            S -.certificates relation.-> P2
            P1 -.peer relation.-> P2
        end
    end

    T[Terraform State] -.tracks.-> C
    T -.tracks.-> M
    T -.tracks.-> P1
    T -.tracks.-> P2
    T -.tracks.-> S

    style C fill:#f3e5f5
    style M fill:#e1f5ff
    style P1 fill:#e8f5e9
    style P2 fill:#e8f5e9
    style S fill:#fff3e0
    style T fill:#fce4ec
:::

Your deployed infrastructure: a Juju controller, a chat model with integrated applications (Mattermost, PostgreSQL with high availability, and TLS certificates), all tracked by Terraform state.
```

```{tip}
**Infrastructure-as-code benefit**: Terraform's state tracking means you can't accidentally create duplicate resources. It knows what exists and only makes necessary changes.
```

## Manage your infrastructure

A key benefit of infrastructure-as-code is that the same workflow handles all changes. Let's scale your PostgreSQL database for improved availability.

On your local workstation, in `2-deploy/main.tf`, modify the `postgresql-k8s` resource to change `units` from `2` to `3`:

```{code-block} terraform
:caption: `2-deploy/main.tf` (partial)

resource "juju_application" "postgresql-k8s" {
  model_uuid = juju_model.chat.uuid

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
:dir: ~/terraform-juju/2-deploy
terraform plan
```

Notice Terraform detected the difference between your desired state (3 units) and actual state (2 units), and shows it will add one unit. This is the power of declarative infrastructure -- you describe what you want, and Terraform figures out how to get there.

```{tip}
**Infrastructure-as-code benefit**: Terraform's state tracking prevents accidental changes. It knows the current state and only makes necessary modifications to reach your desired state.
```

Apply the change:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
:dir: ~/terraform-juju/2-deploy
terraform apply
```

Watch the scaling operation with `juju status --relations`.

On your local workstation, commit your change:

```{terminal}
:copy:
:user:
:host:
git add 2-deploy/main.tf && git commit -m "feat: scale postgresql to 3 units for improved availability"
```

```{tip}
**Infrastructure-as-code benefit**: Your git history now shows why and when you scaled. Anyone on your team can see the evolution of your infrastructure and the reasoning behind each change (captured in commit messages).
```

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

In your VM, destroy the application infrastructure first, then the controller. Start with the deployment directory:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
:dir: ~/terraform-juju/2-deploy
terraform destroy
```

Terraform will show you everything it will remove and ask for confirmation. Type `yes` to proceed.

This removes the applications, integrations, and model. Now destroy the controller:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
:dir: ~/terraform-juju/2-deploy
cd ../1-bootstrap && terraform destroy
```

Type `yes` to confirm. This removes the Juju controller.

Notice how Terraform maintains consistency -- it knows exactly what it created from the state in each directory, so it can cleanly remove everything.

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

