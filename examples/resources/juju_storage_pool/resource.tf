resource "juju_storage_pool" "mypool" {
  name            = "mypool"
  model           = juju_model.development.name
  storageprovider = "tmpfs"
  attributes = {
    a = "b"
    c = "d"
  }
}
