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
}
