resource "juju_model" "this" {
  name = "development" # Model name. Required.

  controller = "overlord" # Controller to operate in. Optional
  cloud {                 # Deploy model to different cloud/region to the controller model. Optional
    name   = "aws"
    region = "eu-west-1"
  }

  logging_config = "<root>=INFO" # Specify log levels. Optional.

  config = { # Override default model configuration. Optional.
    development                 = true
    no-proxy                    = "jujucharms.com"
    update-status-hook-interval = "5m"
    # etc...
  }
}
