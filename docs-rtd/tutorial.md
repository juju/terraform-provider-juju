# Tutorial

<!--
```{toctree}
:maxdepth: 1

self
```
-->

Imagine your business needs a chat service such as Mattermost backed up by a database such as PostgreSQL. In a traditional setup, this can be quite a challenge, but with Juju you'll find yourself deploying, configuring, scaling, integrating, etc., applications in no time. Let's get started!

----------
**What you'll need:**
- A workstation, e.g., a laptop, that has sufficient resources to launch a virtual machine with 4 CPUs, 8 GB RAM, and 50 GB disk space.

**What you'll do:**
- Set up an isolated test environment with Multipass and the `charm-dev` blueprint, which will provide all the necessary tools and configuration for the tutorial (a localhost machine cloud and Kubernetes cloud, Juju, etc.).

- Plan, then deploy, configure, and scale a chat service based on Mattermost and backed by PostgreSQL on a local Kubernetes cloud with Juju.
----------


## Set up an isolated test environment

```{important}

**Tempted to skip this step?** We strongly recommend that you do not! As you will see in a minute, the VM you set up in this step does not just provide you with an isolated test environment but also with almost everything else you’ll need in the rest of this tutorial (and the non-VM alternative may not yield exactly the same results).
```

Follow the instructions for the `juju` CLI.

> See more: {external+juju:ref}`Juju | Set things up <set-things-up>`

In addition to that, on your local workstation, create a directory called `terraform-juju`, then use Multipass to mount it to your Multipass VM. For example, on Linux:

```text
user@ubuntu:~$ mkdir terraform-juju
user@ubuntu:~$ cd terraform-juju/
user@ubuntu:~$ multipass mount ~/terraform-juju my-juju-vm:~/terraform-juju
```

This setup will enable you to create and edit Terraform files in your local editor while running them inside your VM.


## Plan

In this tutorial your goal is to set up a chat service on a cloud.

First, decide which cloud (i.e., anything that provides storage, compute, and networking) you want to use. Juju supports a long list of clouds; in this tutorial we will use a low-ops, minimal production Kubernetes called 'MicroK8s'. In a terminal, open a shell into your VM and verify that you already have MicroK8s installed (`microk8s version`).

> See more: [`juju` | Cloud](https://juju.is/docs/juju/cloud), [`juju` | List of supported clouds](https://juju.is/docs/juju/juju-supported-clouds), [The MicroK8s cloud and Juju](https://juju.is/docs/juju/microk8s), [How to set up your test environment automatically > steps 3-4](https://juju.is/docs/juju/set-up--tear-down-your-test-environment#set-up-tear-down-automatically)

Next, decide which charms (i.e., software operators) you want to use. Charmhub provides a large collection. For this tutorial we will use `mattermost-k8s`  for the chat service,  `postgresql-k8s` for its backing database, and `self-signed-certificates` to TLS-encrypt traffic from PostgreSQL.

> See more: [`juju` | Charm](https://juju.is/docs/juju/charmed-operator), [Charmhub](https://charmhub.io/), Charmhub | [`mattermost-k8s`](https://charmhub.io/mattermost-k8s), [`postgresql-k8s`](https://charmhub.io/postgresql-k8s), [`self-signed-certificates`](https://charmhub.io/self-signed-certificates)


## Deploy, configure, integrate

You will need to install a Juju client; on the client, add your cloud and cloud credentials; on the cloud, bootstrap a controller (i.e., control plan); on the controller, add a model (i.e., canvas to deploy things on; namespace); on the model, deploy, configure, and integrate the charms that make up your chat service.

`terraform-provider-juju` is not self-sufficient -- follow the instructions for the `juju` CLI all the way up to and including the step where you create the  `34microk8s` controller. Also get the details of that controller: `juju show-controller --show-password 34microk8s`.

> See more: [`juju` | Tutorial > Deploy](https://juju.is/docs/juju/tutorial#deploy)

Then, on your VM, install the `terraform` CLI:

```text
ubuntu@my-juju-vm:~$ sudo snap install terraform --classic
terraform 1.7.5 from Snapcrafters✪ installed
```

Next, in your local `terraform-juju` directory, create three files as follows:

(a) a `terraform.tf`file , where you'll configure `terraform` to use the `juju` provider:

```text
terraform {
  required_providers {
    juju = {
      version = "~> 0.11.0"
      source  = "juju/juju"
    }
  }
}
```

(b) a `ca-cert.pem` file, where you'll copy-paste the `ca_certificate` from the details of your `juju`-client-bootstrapped controller; and

(c) a `main.tf` file, where you'll configure the `juju` provider to point to the `juju`-client-bootstrapped controller and the `ca-cert.pem` file where you've saved it's certificate, then create resources to add a model and deploy, configure, and integrate applications:

```terraform
provider "juju" {
   controller_addresses = "10.152.183.27:17070"
   username = "admin"
   password = "40ec19f8bebe353e122f7f020cdb6949"
   ca_certificate = file("~/terraform-juju/ca-cert.pem")
}

resource "juju_model" "chat" {
  name = "chat"
}

resource "juju_application" "mattermost-k8s" {
  model = juju_model.chat.name

  charm {
    name = "mattermost-k8s"
  }

}

resource "juju_application" "postgresql-k8s" {

  model = juju_model.chat.name

  charm {
    name = "postgresql-k8s"
    channel  = "14/stable"
  }

  trust = true

  config = {
    profile = "testing"
   }

}

resource "juju_application" "self-signed-certificates" {
  model = juju_model.chat.name

  charm {
    name = "self-signed-certificates"
  }

}

resource "juju_integration" "postgresql-mattermost" {
  model = juju_model.chat.name

  application {
    name     = juju_application.postgresql-k8s.name
    endpoint = "db"
  }

  application {
    name     = juju_application.mattermost-k8s.name
  }

  # Add any RequiresReplace schema attributes of
  # an application in this integration to ensure
  # it is recreated if one of the applications
  # is Destroyed and Recreated by terraform. E.G.:
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

resource "juju_integration" "postgresql-tls" {
  model = juju_model.chat.name

  application {
    name     = juju_application.postgresql-k8s.name
  }

  application {
    name     = juju_application.self-signed-certificates.name
  }

  # Add any RequiresReplace schema attributes of
  # an application in this integration to ensure
  # it is recreated if one of the applications
  # is Destroyed and Recreated by terraform. E.G.:
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

Next, in your Multipass VM, initialise your provider's configuration (`terraform init`), preview your plan (`terraform plan`), and apply your plan to your infrastructure (`terraform apply`):

```{important}
You can always repeat all three, though technically you only need to run `terraform init` if your `terraform.tf` or the `provider` bit of your `main.tf` has changed, and you only need to run `terraform plan` if you want to preview the changes before applying them.
```

```text
ubuntu@my-juju-vm:~/terraform-juju$ terraform init && terraform plan && terraform apply
```

Finally, use the `juju` client to inspect the results:

```text
ubuntu@my-juju-vm:~/terraform-juju$ juju status --relations
```

Done!

Now, from the output of `juju status`> `Unit` > `mattermost-k8s/0`, retrieve the IP address and the port and feed them to `curl` on the template below:

```text
curl <IP address>:<port>/api/v4/system/ping
```

Sample session:

```text
ubuntu@my-juju-vm:~$ curl 10.1.170.150:8065/api/v4/system/ping
{"ActiveSearchBackend":"database","AndroidLatestVersion":"","AndroidMinVersion":"","IosLatestVersion":"","IosMinVersion":"","status":"OK"}
```

Congratulations, your chat service is up and running!

> See more: [`juju` | How to set up your test environment automatically > steps 3-4](https://juju.is/docs/juju/set-up--tear-down-your-test-environment), {ref}`install-and-manage-terraform-provider-juju`, [`juju` | How to manage clouds](https://juju.is/docs/juju/manage-clouds), {ref}`manage-credentials`, [`juju` | How to manage controllers](https://juju.is/docs/juju/manage-controllers), {ref}`manage-models`, {ref}`manage-applications`


## Scale

A database failure can be very costly. Let's scale it!

On your local machine, in you `main.tf` file, in the definition of the resource for `postgresql-k8s`, add a `units` block and set it to `3`:

```terraform
provider "juju" {
   controller_addresses = "10.152.183.27:17070"
   username = "admin"
   password = "40ec19f8bebe353e122f7f020cdb6949"
   ca_certificate = file("~/terraform-juju/ca-cert.pem")
}

resource "juju_model" "chat" {
  name = "chat"
}


resource "juju_application" "mattermost-k8s" {
  model = juju_model.chat.name

  charm {
    name = "mattermost-k8s"
  }

}


resource "juju_application" "postgresql-k8s" {

  model = juju_model.chat.name

  charm {
    name = "postgresql-k8s"
    channel  = "14/stable"
  }

  trust = true

  config = {
    profile = "testing"
   }

  units = 3

}


resource "juju_application" "self-signed-certificates" {
  model = juju_model.chat.name

  charm {
    name = "self-signed-certificates"
  }

}


resource "juju_integration" "postgresql-mattermost" {
  model = juju_model.chat.name

  application {
    name     = juju_application.postgresql-k8s.name
    endpoint = "db"
  }

  application {
    name     = juju_application.mattermost-k8s.name
  }

  # Add any RequiresReplace schema attributes of
  # an application in this integration to ensure
  # it is recreated if one of the applications
  # is Destroyed and Recreated by terraform. E.G.:
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


resource "juju_integration" "postgresql-tls" {
  model = juju_model.chat.name

  application {
    name     = juju_application.postgresql-k8s.name
  }

  application {
    name     = juju_application.self-signed-certificates.name
  }

  # Add any RequiresReplace schema attributes of
  # an application in this integration to ensure
  # it is recreated if one of the applications
  # is Destroyed and Recreated by terraform. E.G.:
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

Then, in your VM, use `terraform` to apply the changes and `juju` to inspect the results:

```text
ubuntu@my-juju-vm:~/terraform-juju$ terraform init && terraform plan && terraform apply
ubuntu@my-juju-vm:~/terraform-juju$ juju status --relations
```

> See more: {ref}`scale-an-application`


## Tear down your test environment

Follow the instructions for the `juju` CLI.

> See more: {external+juju:ref}`Juju | Tear things down <tear-things-down>`

In addition to that, on your host machine, delete your `terraform-provider-juju` directory.


<!--
## Next steps

This tutorial has introduced you to all the basic things you can do with `terraform-provider-juju`. But there is a lot more to explore:

| If you are wondering... | visit our...                             |
|-------------------------|------------------------------------------|
| "How do I...?"          | [How-to docs](../how-to/index)           |
| "What is...?"           | [Reference docs](../reference/index)     |
| "Why...?", "So what?"   | [Explanation docs](../explanation/index) |
-->