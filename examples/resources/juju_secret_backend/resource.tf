resource "juju_secret_backend" "myvault" {
  name         = "myvault"
  backend_type = "vault"
  config_wo = {
    endpoint = "https://vault.example.com:8200"
    token    = "s.exampletoken"
  }
  config_wo_version    = 1
  token_rotate_interval = "24h"
}
