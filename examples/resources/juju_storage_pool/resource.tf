resource "juju_storage_pool" "mypool" {
  name             = "mypool"
  model_uuid       = juju_model.development.uuid
  storage_provider = "tmpfs"
  attributes = {
    a = "b"
    c = "d"
  }
}
