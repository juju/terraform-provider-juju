resource "juju_integration" "this" {
  model = juju_model.development.name
  via   = "10.0.0.0/24,10.0.1.0/24"

  application {
    name     = juju_application.wordpress.name
    endpoint = "db"
  }

  application {
    name     = juju_application.percona-cluster.name
    endpoint = "server"
  }

  # Add any RequiresReplace schema attributes of
  # an application in this integration to ensure
  # it is recreated if one of the applications
  # is Destroyed and Recreated by terraform. E.G.:
  lifecycle {
    replace_triggered_by = [
      juju_application.wordpress.name,
      juju_application.wordpress.model,
      juju_application.wordpress.constraints,
      juju_application.wordpress.placement,
      juju_application.wordpress.charm.name,
      juju_application.percona-cluster.name,
      juju_application.percona-cluster.model,
      juju_application.percona-cluster.constraints,
      juju_application.percona-cluster.placement,
      juju_application.percona-cluster.charm.name,
    ]
  }
}
