resource "juju_storage_pool" "mypool" {
  name            = "mypool"
  model_uuid      = juju_model.development.uuid
  storageprovider = "tmpfs"
  attributes = {
    a = "b"
    c = "d"
  }
}
