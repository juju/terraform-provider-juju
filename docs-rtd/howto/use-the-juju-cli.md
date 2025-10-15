(use-the-juju-cli-in-terraform)=
# Use the Juju CLI in Terraform

The Terraform Provider for Juju does not support all the functionality of the Juju API.
In order to do things like run Juju actions or wait for application readiness you will need to use the 
Juju CLI within your plan. 

The following sections describe how to use Terraform's 
[provisioners](https://developer.hashicorp.com/terraform/language/provisioners#run-cli-commands) to do 
this for Juju and JAAS. This approach should be viewed as a workaround until the provider develops these capabilities.

See {ref}`create-deployment-dependencies` to combine the provisioner with other tools to wait for application readiness.

(use-the-juju-cli-juju-controller)=
## Juju Controller

When communicating with a Juju controller, the client's filesystem must contain
controller details and credentials commonly located in `~/.local/share/juju` (change this directory
using the [JUJU_DATA](https://documentation.ubuntu.com/juju/3.6/reference/juju-cli/juju-environment-variables/#juju-data)
environment variable).

```terraform
resource "juju_application" "my_charm" {
  name  = ...

  charm {
    ...
  }

  provisioner "local-exec" {
    environment {
      <KEY> = <VALUE>
    }
    command = "juju wait-for application ..."
  }
}
```

(use-the-juju-cli-jaas-controller)=
## JAAS Controller

When communicating with JAAS, the client's filesystem must still contain controller details
but due to differences in JAAS' authentication, credentials must be supplied via environment variables.
From Juju 3.6.12 the following environment variables can be provided to the Juju CLI to 
authenticate with JAAS using a service account.

```terraform
resource "juju_application" "my_charm" {
  name  = ...

  charm {
    ...
  }

  provisioner "local-exec" {
    environment {
      JUJU_CLIENT_ID     = <VALUE>
      JUJU_CLIENT_SECRET = <VALUE>
    }
    command = "juju wait-for application ..."
  }
}
```
