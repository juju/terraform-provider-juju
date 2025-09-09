terraform {
  required_providers {
    juju = {
      source  = "juju/juju"
      version = ">= 0.15.0"
    }
    github = {
      source  = "hashicorp/github"
      version = "6.6.0"
    }
  }

  required_version = ">= 1.5.0"
}

terraform {
  required_providers {
    github = {
      source  = "hashicorp/github"
      version = "6.6.0"
    }
    juju = {
      source  = "juju/juju"
      version = ">= 0.15.0"
    }
  }
  required_version = ">= 1.5.0"
}

terraform {
  required_providers {
    juju = {
      source  = "juju/juju"
      version = ">= 0.15.0"
    }
  }
  required_version = ">= 1.5.0"
}
