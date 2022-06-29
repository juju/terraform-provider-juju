resource "juju_integration" "this" {
  model = "development"

  application {
    name     = "wordpress"
    endpoint = "db" # can be optional
  }

  application {
    name     = "percona-cluster"
    endpoint = "server" # can be optional
  }
}
