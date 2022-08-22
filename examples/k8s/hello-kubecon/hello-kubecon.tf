# This is an example of how to deploy the hello-kubecon charm
# with its own nginx ingress. 
# Check https://charmhub.io/hello-kubecon for more details.
#
# This example deploys both charms and integrates them.
# Observe how the nginx charm is configured to point to the
# hello-kubecon internal service and sets an external
# hostname.
#
# After deployed, check that everything is running with
# a request using the corresponding host header:
# `curl -H "Host: service.foo" 127.0.0.1`
# If you'd rather want to see the corresponding page in your
# browser you can run:
# echo "127.0.1.1 service.foo" | sudo tee -a /etc/hosts
# and then visit http://service.foo


terraform {
  required_providers {
    juju = {
      source  = "juju/juju"
      version = "0.4.0"
    }
  }
}

provider "juju" {}

resource "juju_model" "app" {
  name = "hello-kubecon"
  config = {
    logging-config = "<root>=INFO;unit=DEBUG"
  }
}

resource "juju_application" "hello_kubecon" {
  name  = "hello-kubecon"
  model = juju_model.app.name
  charm {
    name = "hello-kubecon"
  }
}

resource "juju_application" "nginx" {
  name  = "ingress"
  model = juju_model.app.name
  charm {
    name = "nginx-ingress-integrator"
  }
  config = {
    service-name     = juju_application.hello_kubecon.name
    service-port     = "80"
    service-hostname = "service.foo"
  }

  trust = true
}

resource "juju_integration" "endpoint" {
  model = juju_model.app.name

  application {
    name     = juju_application.hello_kubecon.name
    endpoint = "ingress"
  }

  application {
    name     = juju_application.nginx.name
    endpoint = "ingress"
  }
}
