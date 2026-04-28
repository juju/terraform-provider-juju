---
myst:
  html_meta:
    description: "Learn to manage Juju infrastructure as code with the Terraform Provider, using declarative configuration files, version control, and infrastructure-as-code workflows."
---

# Get started with the Terraform Provider for Juju

The Terraform Provider for Juju brings infrastructure-as-code capabilities to {external+juju:doc}`Juju <index>`.

In this tutorial you will define and deploy a chat service (Mattermost backed by PostgreSQL) using declarative configuration files managed with Terraform.

**What you'll need:**

- A workstation, e.g., a laptop, that has sufficient resources to launch a virtual machine with 4 CPUs, 8 GB RAM, and 50 GB disk space.
- Familiarity with a terminal.
- Basic familiarity with Juju concepts (controllers, models, charms, applications).
- Basic familiarity with Terraform (providers, resources, state).

**What you'll do:**

- Set things up: launch a Juju-ready VM using Multipass, install Terraform, and bootstrap a Juju controller.
- See how to manage users and access control as code.
- Deploy infrastructure and applications with Terraform configuration files.
- Clean up resources.

## Set things up

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

You'll have everything you need in an isolated environment.
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
```

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
juju version
```

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
juju clouds --client
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
sudo sh -c "mkdir -p /var/snap/terraform/current/microk8s/credentials"
```

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
sudo sh -c "microk8s config > /var/snap/terraform/current/microk8s/credentials/client.config"
```

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
sudo chown -f -R $USER:$USER /var/snap/terraform/current/microk8s/credentials/client.config
```

```{dropdown} Why is this necessary?

When the Terraform Provider bootstraps the controller, it needs access to the MicroK8s credentials. This command copies the credentials to a location where they can be found during the bootstrap process.
```

Now, on your local workstation (not the VM), create a directory for your Terraform configuration:

```{terminal}
:copy:
:user:
:host:
mkdir ~/terraform-juju
```

```{terminal}
:copy:
:user:
:host:
cd ~/terraform-juju
```

Mount it to your VM:

```{terminal}
:copy:
:user:
:host:
multipass mount ~/terraform-juju my-juju-vm:terraform-juju
```

This lets you create files locally and run Terraform on them inside the VM, while using your IDE to view and edit the files.

```{tip}
**Recommended workflow setup:**

You'll be working across two contexts: your local workstation and the VM. To work efficiently:

1. **Two terminal windows (or one split terminal):**
   - One terminal for your local workstation (where you'll create files, run `multipass` commands, and run `git` commands)
   - One terminal for the VM shell (where you'll run `terraform` and `juju` commands)

2. **Your favorite text editor** on your local workstation to view and edit `.tf` files. Changes you make locally will be automatically visible in the VM via the mounted directory.
```

Initialize version control. On your local workstation, in your `terraform-juju` directory:

```{terminal}
:copy:
:user:
:host:
git init
```

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

Now bootstrap a Juju controller. A Juju controller is your Juju control plane -- the entity that holds the Juju API server and Juju's database. With the Terraform Provider, you can bootstrap a controller by defining it in your Terraform configuration.

On your VM, view the MicroK8s credentials that you'll need for your Terraform configuration:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
juju show-credential microk8s microk8s --show-secrets

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

On your local workstation, in your `terraform-juju` directory, create your controller bootstrap configuration.

First, create `1-bootstrap/terraform.tf` to configure Terraform to use the Juju provider in controller mode:

```{terminal}
:copy:
:user:
:host:
:dir: ~/terraform-juju
touch 1-bootstrap/terraform.tf
```

```{code-block} terraform
:caption: `1-bootstrap/terraform.tf`

terraform {
  required_providers {
    juju = {
      version = "~> 1.4"
      source  = "juju/juju"
    }
    local = {
      source = "hashicorp/local"
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

```{terminal}
:copy:
:user:
:host:
:dir: ~/terraform-juju
touch 1-bootstrap/variables.tf
```

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

```{terminal}
:copy:
:user:
:host:
:dir: ~/terraform-juju
touch 1-bootstrap/terraform.tfvars
```

```{code-block} terraform
:caption: `1-bootstrap/terraform.tfvars`

k8s_token    = "eyJhbGciOiJSUzI1NiIsImtpZCI6IldBbERh..."
k8s_ca_cert  = "LS0tLS1CRUdJTi..."
k8s_endpoint = "https://10.x.x.x:16443"
```

```{note}
The values shown above are examples only. Use your actual values from the previous commands -- the token and certificate will be much longer than shown here.
```

Before continuing, keep credentials, connection info, and Terraform state safe and out of version control.

Create `1-bootstrap/.gitignore`:

```{terminal}
:copy:
:user:
:host:
:dir: ~/terraform-juju
touch 1-bootstrap/.gitignore
```

```{code-block} text
:caption: `1-bootstrap/.gitignore`

terraform.tfvars
conn_info.json
.terraform*
terraform.tfstate*
```

Now create `1-bootstrap/main.tf` to define your controller:

```{terminal}
:copy:
:user:
:host:
:dir: ~/terraform-juju
touch 1-bootstrap/main.tf
```

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

To allow the deployment configuration to connect to this controller, add a resource that writes the connection details to a JSON file. Add this to `1-bootstrap/main.tf`:

```{code-block} terraform
:caption: `1-bootstrap/main.tf` (add after juju_controller resource)

resource "local_file" "conn_info" {
  filename = "${path.module}/conn_info.json"
  content = jsonencode({
    addresses = juju_controller.microk8s.api_addresses
    username  = juju_controller.microk8s.username
    password  = juju_controller.microk8s.password
    ca_cert   = juju_controller.microk8s.ca_cert
  })
}
```

On your local workstation, in your `terraform-juju` directory, commit your controller infrastructure definition (excluding sensitive files):

```{terminal}
:copy:
:user:
:host:
:dir: ~/terraform-juju
git add 1-bootstrap && git commit -m "feat: define Juju controller infrastructure"
```

Now, in your VM, initialize Terraform in the bootstrap directory. If you exited the VM shell, reopen it:

```{terminal}
:copy:
:user:
:host:
multipass shell my-juju-vm
```

Initialize the bootstrap directory:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform -chdir=terraform-juju/1-bootstrap init
```

This downloads the Juju provider plugin and prepares your workspace.

Preview what Terraform will create:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform -chdir=terraform-juju/1-bootstrap plan
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
terraform -chdir=terraform-juju/1-bootstrap apply
```

Terraform will show you the plan again and ask for confirmation. Type `yes` to proceed.

```{note}
The bootstrap process typically takes 1-2 minutes, but may vary depending on your system and network speed. Terraform will show progress as it creates the controller.
```

Once complete, your Juju controller is running on MicroK8s, and Terraform has recorded its state. Terraform has also created `1-bootstrap/conn_info.json` with the controller's connection details (minified JSON on one line; to view formatted, run `cat terraform-juju/1-bootstrap/conn_info.json | jq .`).

## Handle authentication and authorization

When you bootstrap a controller, Juju automatically creates an admin user with superuser access. This user's credentials are available in the connection info JSON file created by the bootstrap plan.

While you could manage users via the `juju` CLI, the Terraform Provider lets you manage users as code using `juju_user` resources. You can also manage access control for users across controllers, models, and other resources using `juju_access_*` resources.

> See more: {ref}`Manage users <manage-users>`, {ref}`Manage access to a controller <manage-access-to-a-controller>`, {ref}`Manage access to a model <manage-access-to-a-model>`

For this tutorial, you'll use the admin user that was automatically created during bootstrap. The deployment configuration will read this user's controller login credentials from the `conn_info.json` file.

## Deploy infrastructure and applications

With your controller bootstrapped, you'll now define and deploy the applications that make up your chat service. You'll deploy Mattermost for the chat service, PostgreSQL for its backing database, and self-signed certificates to TLS-encrypt traffic from PostgreSQL.

To connect to your bootstrapped controller, you'll read its connection details from the JSON file created by the bootstrap plan. This file contains the controller's API addresses, credentials, and CA certificate.

On your local workstation, in your `terraform-juju` directory, create your application deployment configuration.

First, create `2-deploy/terraform.tf` to configure the provider to connect to your controller:

```{terminal}
:copy:
:user:
:host:
:dir: ~/terraform-juju
touch 2-deploy/terraform.tf
```

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

# Read connection details from the bootstrap JSON file
locals {
  conn_info = jsondecode(file("${path.module}/../1-bootstrap/conn_info.json"))
}

provider "juju" {
  controller_addresses = join(",", local.conn_info.addresses)
  username             = local.conn_info.username
  password             = local.conn_info.password
  ca_certificate       = local.conn_info.ca_cert
}
```

```{important}
Notice there's no `controller_mode` setting (it defaults to `false`). This configuration can manage models, applications, and integrations, but cannot manage `juju_controller` resources.

The provider connects to the bootstrapped controller by reading connection details from the JSON file created by the bootstrap plan. This separation is necessary because Terraform cannot configure a provider from resource outputs in the same plan.
```

Create `2-deploy/.gitignore` to keep Terraform state out of version control:

```{terminal}
:copy:
:user:
:host:
:dir: ~/terraform-juju
touch 2-deploy/.gitignore
```

```{code-block} text
:caption: `2-deploy/.gitignore`

.terraform*
terraform.tfstate*
```

Now create `2-deploy/main.tf` to define your application resources:

```{terminal}
:copy:
:user:
:host:
:dir: ~/terraform-juju
touch 2-deploy/main.tf
```

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

# Define the database
resource "juju_application" "postgresql-k8s" {
  model_uuid = juju_model.chat.uuid

  charm {
    name    = "postgresql-k8s"
    channel = "14/stable"
  }

  trust = true
  units = 1

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

Notice how this declarative definition makes your infrastructure intentions clear: you want a chat model with Mattermost, PostgreSQL, TLS certificates, and specific integrations between them.

```{note}
Notice the explicit `endpoint` values in the TLS integration -- this ensures the correct relation endpoints are used.
```

On your local workstation, in your `terraform-juju` directory, commit your application infrastructure definition:

```{terminal}
:copy:
:user:
:host:
:dir: ~/terraform-juju
git add 2-deploy && git commit -m "feat: define chat application infrastructure"
```

Now deploy your infrastructure. In your VM, initialize and preview the deployment:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform -chdir=terraform-juju/2-deploy init
```

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform -chdir=terraform-juju/2-deploy plan
```

This shows what Terraform will create without actually creating anything. Review the output carefully -- you'll see the model, applications, and integrations that will be created.

Apply your infrastructure definition:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform -chdir=terraform-juju/2-deploy apply
```

Terraform will show you the plan again and ask for confirmation. Type `yes` to proceed.

The deployment will take a few minutes. Terraform will show you the progress as it creates each resource.

Once complete, verify your applications are running. Wait for all pods to be ready:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
microk8s kubectl wait --for=condition=ready pod -l operator.juju.is/name -n chat --timeout=300s
```

The Mattermost charm needs time to complete database setup before creating its workload pod. Monitor the pods until the `mattermost-k8s-0` pod appears and shows `Running` status:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
watch microk8s kubectl get pods -n chat
```

Press {kbd}`Ctrl` + {kbd}`C` to exit once all pods show `Running`. This may take several minutes.

Once all pods are running, get the service address:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
microk8s kubectl get svc -n chat mattermost-k8s -o jsonpath='{.spec.clusterIP}:{.spec.ports[0].port}'
```

Test your chat service (replace `<address>` with the output from above):

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
curl <address>/api/v4/system/ping
```

Sample output:

```text
{"ActiveSearchBackend":"database","AndroidLatestVersion":"","AndroidMinVersion":"","IosLatestVersion":"","IosMinVersion":"","status":"OK"}
```

Congratulations! Your chat service is up and running, and your entire infrastructure is defined as code.

```{tip}
**Infrastructure-as-code benefit**: Terraform's state tracking means you can't accidentally create duplicate resources. It knows what exists and only makes necessary changes.
```

To build on what you've learned: configure controller and model settings during bootstrap ({ref}`configure-a-controller`), manage controllers post-bootstrap ({ref}`manage-controllers`), use Terraform workspaces for multiple environments ({ref}`manage-models`), integrate with other cloud providers, set up remote state storage for team collaboration, and explore all provider features ({ref}`reference`).

## Tear things down

With Terraform, tearing down your infrastructure is as simple as deploying it.

In your VM, destroy the application infrastructure first, then the controller:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform -chdir=terraform-juju/2-deploy destroy
```

Terraform will show you everything it will remove and ask for confirmation. Type `yes` to proceed.

This removes the applications, integrations, and model. Now destroy the controller:

```{terminal}
:copy:
:user: ubuntu
:host: my-juju-vm
terraform -chdir=terraform-juju/1-bootstrap destroy
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

