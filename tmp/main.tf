terraform {
  required_providers {
    juju = {
      version = "1.1.0"
      source  = "juju/juju"
    }
  }
}

locals {
  root_user_priv  = pathexpand("~/.local/share/juju/ssh/juju_id_rsa")
  ubuntu_user_pub = pathexpand("~/.ssh/id_ed25519.pub")
  ubuntu_user_priv= pathexpand("~/.ssh/id_ed25519")
}

provider "juju" {}

resource "juju_model" "test_model" {
  name = "issue929"
}

# Create machine with root user (not setup required, just address and the private key & ubuntu users public key)
resource "juju_machine" "manual" {
  model_uuid       = juju_model.test_model.uuid
  ssh_address      = "root@10.170.115.166"
  private_key_file = local.root_user_priv
  public_key_file  = local.ubuntu_user_pub
}

# Create machine with ubuntu user already setup (no keys required, as the "provisioning segment" has been completed)
# And the private key for the ubuntu users should be in ~/.ssh. 
resource "juju_machine" "manual2" {
  model_uuid       = juju_model.test_model.uuid
  ssh_address      = "ubuntu@10.170.115.103"
  # Not used.
  private_key_file = local.ubuntu_user_priv
  # Not used.
  public_key_file  = local.ubuntu_user_pub
}