# Test file for juju_ssh_key resource transformations
data "juju_model" "existing" {
  name = "existing-model"
}

resource "juju_model" "test" {
  name = "test-model"
}

# juju_ssh_key with resource reference (should be upgraded)
resource "juju_ssh_key" "key1" {
  model   = juju_model.test.name
  payload = "ssh-rsa AAAAB3NzaC1yc2E..."
}

# juju_ssh_key with data source reference (should be upgraded)
resource "juju_ssh_key" "key2" {
  model   = data.juju_model.existing.name
  payload = "ssh-rsa AAAAB3NzaC1yc2E..."
}

# juju_ssh_key already using model_uuid (should NOT be upgraded)
resource "juju_ssh_key" "already_correct" {
  model_uuid = juju_model.test.uuid
  payload    = "ssh-rsa AAAAB3NzaC1yc2E..."
}

# juju_ssh_key with variable reference (should NOT be upgraded)
resource "juju_ssh_key" "with_variable" {
  model   = var.model_name
  payload = "ssh-rsa AAAAB3NzaC1yc2E..."
}
