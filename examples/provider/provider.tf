provider "juju" {
  controller_addresses = "10.225.205.241:17070,10.225.205.242:17070"

  username = "jujuuser"
  password = "password1"

  ca_certificate = file("~/ca-cert.pem")
}
