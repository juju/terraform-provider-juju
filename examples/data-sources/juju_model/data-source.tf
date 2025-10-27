# Reference a model by name and owner
data "juju_model" "this" {
  owner = "admin"
  name  = "database"
}

# Reference a model by UUID
data "juju_model" "this" {
  uuid = "1d10a751-02c1-43d5-b46b-d84fe04d6fde"
}
