resource "juju_integration" "this" {
  model = "development"

  application {
    name     = "wordpress"
    endpoint = "db"
  }

  application {
    name     = "percona-cluster"
    endpoint = "server"
  }
}
