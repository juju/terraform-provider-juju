resource "juju_credential" "this" {
  name = "creddev"

  cloud {
    name = "localhost"
  }

  auth_type = "certificate"

  attributes = {
    client-cert    = "/srv/cert.crt"
    client-key     = "/srv/cert.key"
    trust-password = "S0m3P@$$w0rd"
  }
}
