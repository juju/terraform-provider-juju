# This example deploys Grafana using Prometheus
# as a data source.
# Check https://charmhub.io/grafana-k8s for more details.
#
# After the deployment, get the admin password running:
# `juju run-action grafana/0 get-admin-password --wait`
#
# Next, browse to the main Grafana login page. Use the 
# grafana/0 unit IP and port 3000 (http://10.1.250.216:3000)
# and introduce the user (admin) and password provided in 
# the previous step. Check that the prometheus datasource is 
# already available and you can use it.


terraform {
  required_providers {
    juju = {
      source  = "juju/juju"
      version = "0.4.0"
    }
  }
}


provider "juju" {}

# A model for grafana
resource "juju_model" "mygrafana" {
  name = "mygrafana"
}

# Grafana charm
resource "juju_application" "grafana" {
  name  = "grafana"
  model = juju_model.mygrafana.name
  charm {
    name    = "grafana-k8s"
    channel = "latest/beta"
  }
}

# Prometheus charm
resource "juju_application" "prometheus" {
  name  = "prometheus"
  model = juju_model.mygrafana.name
  charm {
    name    = "prometheus-k8s"
    channel = "latest/candidate"
  }
  trust = true
}

# Integrate Prometheus with Grafana
resource "juju_integration" "prometheus_grafana" {
  model = juju_model.mygrafana.name
  application {
    name     = juju_application.grafana.name
    endpoint = "grafana-source"
  }
  application {
    name     = juju_application.prometheus.name
    endpoint = "grafana-source"
  }
}
