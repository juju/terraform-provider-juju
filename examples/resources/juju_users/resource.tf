resource "juju_user" "this" {
  name         = "dev-user"
  display_name = format("%s - terraform managed", "dev-user")
  password     = var.password
}
