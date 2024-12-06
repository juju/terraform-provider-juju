(manage-machines)=
# How to manage machines

<!--FIGURE OUT A GOOD PLACE FOR THIS:
An interactive pseudo-terminal (pty) is enabled by default. For the OpenSSH client, this corresponds to the `-t` option ("force pseudo-terminal allocation").

Remote commands can be run as expected. For example: `juju ssh 1 lsb_release -c`. For complex commands the recommended method is by way of the `run` command.
-->

> See also: [`juju` | Machine](https://juju.is/docs/juju/machine)


## Add a machine

To add a machine to a model, in your Terraform plan add a resource of the `juju_machine` type, specifying the model. 

```terraform
resource "juju_machine" "machine_0" {
  model       = juju_model.development.name
}
```

You can optionally specify a base, a name, regular constraints, storage constraints, etc. You can also specify a `private_key_file`, `public_key_file`, and `ssh_address` -- that will allow you to add to the model an existing, manual machine (rather than a virtual one provisioned for you by the cloud).


> See more: [`juju_machine` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/machine)

## Manage constraints for a machine
> See also: [`juju` | Constraint](https://juju.is/docs/juju/constraint)

To set constraints for a machine, in your Terraform plan, in the machine resource definition, set the constraints attribute to the desired quotes-enclosed, space separated list of key=value pairs. For example:

```terraform
resource "juju_machine" "machine_0" {
  model       = juju_model.development.name
  name        = "machine_0"
  constraints = "tags=my-machine-tag"
}
```

> See more: [`juju_machine` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/machine)



## Remove a machine
> See also: [`juju` | Removing things](https://juju.is/docs/juju/removing-things)

To remove a machine, remove its resource definition from your Terraform plan.

> See more: [`juju_machine` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/machine)


<br>

> <small>**Contributors:** @alhama7a, @cderici, @tmihoc </small>
