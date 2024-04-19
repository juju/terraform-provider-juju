data "juju_model" "my_model" {
  name = "default"
}

data "juju_secret" "my_secret_data_source" {
  name  = "my_secret"
  model = data.juju_model.my_model.name
}

resource "juju_application" "ubuntu" {
  model = juju_model.my_model.name
  name  = "ubuntu"

  charm {
    name = "ubuntu"
  }

  config = {
    secret = data.juju_secret.my_secret_data_source.secret_id
  }
}

resource "juju_access_secret" "my_secret_access" {
  model = juju_model.my_model.name
  applications = [
    juju_application.ubuntu.name
  ]
  secret_id = data.juju_secret.my_secret_data_source.secret_id
}

