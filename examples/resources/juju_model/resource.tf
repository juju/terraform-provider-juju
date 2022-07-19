resource "juju_model" "this" {
  name = "development"

  cloud {
    name   = "aws"
    region = "eu-west-1"
  }

  config = {
    logging-config              = "<root>=INFO"
    development                 = true
    no-proxy                    = "jujucharms.com"
    update-status-hook-interval = "5m"
  }
}
