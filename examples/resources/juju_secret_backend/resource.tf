resource "juju_secret_backend" "myvault" {
  name         = "myvault"
  backend_type = "vault"
  config = {
    endpoint = "https://vault.example.com:8200"
    token    = "s.exampletoken"
  }
  token_rotate_interval = "24h"
}
