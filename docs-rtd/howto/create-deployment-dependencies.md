(create-deployment-dependencies)=
# Create deployment dependencies

> See also: {external+juju:ref}`Juju | Charm <charm>`

(create-a-dependency)=
## Create a dependency

The Terraform Provider for Juju does not support waiting for a particular charm status before
creating other resources. However, you can use Terraform native
[provisioner local-exec](local-exec-ref), Terraform's [null_resource](null-resource-ref), Juju
CLI's [`wait-for` command](juju-wait-for-ref) altogether to create a dependency in resource
creations.
This is particularly useful when the charm may not be holistic and requires a step-by-step approach
in deployment strategies.

To create a dependency, you can use the `null_resource` resource to run a command that waits for
the charm to be active before starting the integration.

```terraform
resource "juju_application" "my_charm" {
  name  = ...

  charm {
    ...
  }
}

resource "null_resource" "wait_for_my_charm" {
  provisioner "local-exec" {
    command = "juju wait-for application ${juju_application.my_charm.name}"
  }
}

resource "juju_integration" "my_integration" {
    depends_on = [ null_resource.wait_for_my_charm ]
}
```

This way, the `my_integration` resource will only be created after the `my_charm` application is
active, ensuring that the charm is ready for integration.

[local-exec-ref](https://developer.hashicorp.com/terraform/language/provisioners#commands-on-the-local-machine)
[null-resource-ref](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource)
[juju-wait-for-ref](https://documentation.ubuntu.com/juju/3.6/reference/juju-cli/list-of-juju-cli-commands/wait-for/)
