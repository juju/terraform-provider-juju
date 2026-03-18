list "juju_storage_pool" "this" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = "<model-uuid>"

    # Optional: filter by storage pool name
    # name = "my-storage-pool"

    # Optional: filter by storage provider
    # storage_provider = "ebs"
  }
}
