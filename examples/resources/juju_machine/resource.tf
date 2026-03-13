# A manually provisioned machine based on any provider other than the manual provider.
resource "juju_machine" "this_machine" {
  model_uuid  = juju_model.development.uuid
  base        = "ubuntu@22.04"
  name        = "this_machine"
  constraints = "tags=my-machine-tag"

  # If you face timeouts destroying machines, add the following lifecycle
  # directive, which instructs Terraform to update any dependent resources
  # before destroying the machine - in the case of applications this means
  # that application units get removed from units before Terraform attempts
  # to destroy the machine.
  lifecycle {
    create_before_destroy = true
  }
}


# A manually provisioned machine using a provisioning user to setup the ubuntu user.
locals {
  provisioning_user_priv = pathexpand("~/path/to/provisioning/user/key")
  ubuntu_user_pub        = pathexpand("~/path/to/ubuntu/user/key.pub")
}

resource "juju_machine" "manual_machine" {
  model_uuid       = juju_model.test_model.uuid
  ssh_address      = "provisioning-user@1.1.1.1"
  private_key_file = local.provisioning_user_priv
  public_key_file  = local.ubuntu_user_pub
}
