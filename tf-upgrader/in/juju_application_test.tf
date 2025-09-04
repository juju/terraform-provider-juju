# Test file for juju_application resource transformations
data "juju_model" "production" {
  name = "production-env"
}

resource "juju_model" "development" {
  name = "dev-environment"
}

# juju_application with resource reference (should be upgraded)
resource "juju_application" "database" {
  name = "postgresql"
  charm {
    name = "postgresql"
  }
  model = juju_model.development.name
  units = 1
}

# juju_application with data source reference (should be upgraded)
resource "juju_application" "monitoring" {
  name = "prometheus"
  charm {
    name = "prometheus-k8s"
  }
  model = data.juju_model.production.name
}

# juju_application already using model_uuid (should NOT be upgraded)
resource "juju_application" "already_correct" {
  name = "grafana"
  charm {
    name = "grafana-k8s"
  }
  model_uuid = juju_model.development.uuid
}

# juju_application with variable reference (should NOT be upgraded)
resource "juju_application" "with_variable" {
  name = "mysql"
  charm {
    name = "mysql"
  }
  model = var.model_name
}
