resource "juju_integration" "this" {
  model = "development"

  application {
    name        = "wordpress"
    integration = "db" # can be optional
  }

  application {
    name        = "percona-cluster"
    integration = "server" # can be optional
  }
}
