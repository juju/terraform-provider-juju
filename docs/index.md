---
# generated using template templates/index.md.tmpl
page_title: "Juju Provider"
subcategory: ""
description: |-
  
---

# Juju Provider

The provider can be used to interact with [Juju][0] - an open source orchestration engine by Canonical.

The provider only interacts with a single controller at a time.

Today this provider allows you to manage the following via resources:

* Applications and deploy charms
* Credentials for existing clouds
* Integrations
* Machines
* Models
* Model ssh keys
* Offers
* Users

and refer to the following via data sources:

* Machines
* Models
* Offers

Work is ongoing to include support for more of the juju CLIs capabilities within this provider.

## Prerequisites

* [Juju][0] `2.9.49+`

## Authentication

There are 3 ways to define credentials for authentication with the Juju controller you wish to target.
They are displayed in the order in which the provider looks for credentials.

### Static credentials

Define the Juju controller credentials in the provider definition in your terraform plan.

``` terraform
provider "juju" {
  controller_addresses = "10.225.205.241:17070,10.225.205.242:17070"
  username = "jujuuser"
  password = "password1"
  ca_certificate = file("~/ca-cert.pem")
}
```

### Client credentials

Define the client credentials in the provider definition in your terraform plan.

``` terraform
provider "juju" {
  controller_addresses = "10.225.205.241:17070,10.225.205.242:17070"
  client_id = "jujuclientid"
  client_secret = "jujuclientsecret"
  ca_certificate = file("~/ca-cert.pem")
}
```

### Environment variables

Define the Juju controller credentials in the provider definition via environment variables. These can be set up as follows:

```shell
export CONTROLLER=$(juju whoami | yq .Controller)
export JUJU_CONTROLLER_ADDRESSES="$(juju show-controller | yq '.[$CONTROLLER]'.details.\"api-endpoints\" | tr -d "[]' "|tr -d '"'|tr -d '\n')"
export JUJU_USERNAME="$(cat ~/.local/share/juju/accounts.yaml | yq .controllers.$CONTROLLER.user|tr -d '"')"
export JUJU_PASSWORD="$(cat ~/.local/share/juju/accounts.yaml | yq .controllers.$CONTROLLER.password|tr -d '"')"
export JUJU_CA_CERT="$(juju show-controller $(echo $CONTROLLER|tr -d '"') | yq '.[$CONTROLLER]'.details.\"ca-cert\"|tr -d '"'|sed 's/\\n/\n/g')"
```

### Populated by the provider via the juju CLI client.

This is the most straightforward solution. Remember that it will use the configuration used by the Juju CLI client at that moment. The fields are populated using the
 output from running the command `juju show-controller` with the `--show-password` flag.

## Example Usage

Terraform 0.13 and later:
```terraform
terraform {
  required_providers {
    juju = {
      version = "~> 0.12.0"
      source  = "juju/juju"
    }
  }
}

provider "juju" {}

resource "juju_model" "development" {
  name = "development"

  cloud {
    name   = "aws"
    region = "eu-west-1"
  }
}

resource "juju_application" "wordpress" {
  name = "wordpress"

  model = juju_model.development.name

  charm {
    name = "wordpress"
  }

  units = 3
}

resource "juju_application" "percona-cluster" {
  name = "percona-cluster"

  model = juju_model.development.name

  charm {
    name = "percona-cluster"
  }

  units = 3
}

resource "juju_integration" "wp_to_percona" {
  model = juju_model.development.name

  application {
    name     = juju_application.wordpress.name
    endpoint = "db"
  }

  application {
    name     = juju_application.percona-cluster.name
    endpoint = "server"
  }
}
```

Terraform 0.12 and earlier:
```terraform
provider "juju" {
  version = "~> 0.12.0"

  controller_addresses = "10.225.205.241:17070,10.225.205.242:17070"

  username = "jujuuser"
  password = "password1"

  ca_certificate = file("~/ca-cert.pem")
}

resource "juju_model" "development" {
  name = "development"

  cloud {
    name   = "aws"
    region = "eu-west-1"
  }
}

resource "juju_application" "wordpress" {
  name = "wordpress"

  model = juju_model.development.name

  charm {
    name = "wordpress"
  }

  units = 3
}

resource "juju_application" "percona-cluster" {
  name = "percona-cluster"

  model = juju_model.development.name

  charm {
    name = "percona-cluster"
  }

  units = 3
}

resource "juju_integration" "wp_to_percona" {
  model = juju_model.development.name

  application {
    name     = juju_application.wordpress.name
    endpoint = "db"
  }

  application {
    name     = juju_application.percona-cluster.name
    endpoint = "server"
  }
}
```

Terraform 0.12 and later with client credentials:
```terraform
provider "juju" {
  version = "~> 0.10.0"

  controller_addresses = "10.225.205.241:17070,10.225.205.242:17070"

  client_id     = "jujuclientid"
  client_secret = "jujuclientsecret"

  ca_certificate = file("~/ca-cert.pem")
}

resource "juju_model" "development" {
  name = "development"

  cloud {
    name   = "aws"
    region = "eu-west-1"
  }
}

resource "juju_application" "wordpress" {
  name = "wordpress"

  model = juju_model.development.name

  charm {
    name = "wordpress"
  }

  units = 3
}

resource "juju_application" "percona-cluster" {
  name = "percona-cluster"

  model = juju_model.development.name

  charm {
    name = "percona-cluster"
  }

  units = 3
}

resource "juju_integration" "wp_to_percona" {
  model = juju_model.development.name

  application {
    name     = juju_application.wordpress.name
    endpoint = "db"
  }

  application {
    name     = juju_application.percona-cluster.name
    endpoint = "server"
  }
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `ca_certificate` (String) This is the certificate to use for identification. This can also be set by the `JUJU_CA_CERT` environment variable
- `client_id` (String) This is the client ID to be used. This can also be set by the `JUJU_CLIENT_ID` environment variable
- `client_secret` (String, Sensitive) This is the client secret to be used. This can also be set by the `JUJU_CLIENT_SECRET` environment variable
- `controller_addresses` (String) This is the Controller addresses to connect to, defaults to localhost:17070, multiple addresses can be provided in this format: <host>:<port>,<host>:<port>,.... This can also be set by the `JUJU_CONTROLLER_ADDRESSES` environment variable.
- `password` (String, Sensitive) This is the password of the username to be used. This can also be set by the `JUJU_PASSWORD` environment variable
- `username` (String) This is the username registered with the controller to be used. This can also be set by the `JUJU_USERNAME` environment variable


[0]: https://juju.is "Juju | An open source application orchestration engine"
