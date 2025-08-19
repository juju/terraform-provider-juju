data "juju_model" "my_model" {
  name = "default"
}

data "juju_secret" "my_secret_data_source" {
  name  = "my_secret"
  model = data.juju_model.my_model.name
}

resource "juju_application" "ubuntu" {
  model_uuid = data.juju_model.my_model.uuid
  name       = "ubuntu"

  charm {
    name = "ubuntu"
  }

  config = {
    secret = data.juju_secret.my_secret_data_source.secret_id
  }
}

resource "juju_access_secret" "my_secret_access" {
  model_uuid = data.juju_model.my_model.uuid
  applications = [
    juju_application.ubuntu.name
  ]
  secret_id = data.juju_secret.my_secret_data_source.secret_id
}

