terraform {
  required_providers {
    juju = {
      version = "0.20.0"
      source  = "juju/juju"
    }
  }
}


# All model level operations require a model resource to be defined.
# create-storage-pool
# update-storage-pool 
# list-storage-pools
# remove-storage-pool 
provider "juju" {}

resource "juju_model" "example_model" {
  name = "example-model"
	constraints = "arch=arm64"
}

# resource "juju_storage_pool" "example_pool" {
#   name       = "example-pool"
#   model_id   = juju_model.example_model.id
#   provider       = "tmpfs" # There's 3 for all clouds, but some clouds have specific ones (i.e., ebs aws)
#   configuration = { # Juju docs refer to these key/values as configuration, but pool listing calls them attributes...
#     pool_name = "example-pool"
#   }
# }
