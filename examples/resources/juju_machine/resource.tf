resource "juju_machine" "this_machine" {
  model       = juju_model.development.name
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
