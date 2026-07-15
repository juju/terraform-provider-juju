---
myst:
  html_meta:
    description: "Learn how to create, reference, list, and remove Juju spaces and assign subnets to spaces using the Terraform Provider for Juju."
---

(manage-spaces)=
# Manage spaces

> See also: {external+juju:ref}`Juju | Space <space>`, {external+juju:ref}`Juju | Subnet <subnet>`

A Juju **space** is a logical grouping of subnets that can be referenced by name when configuring application endpoint bindings. In the Terraform Provider for Juju, the `juju_subnet` resource does not create a subnet — it acts as an **assignment tool** that assigns an existing subnet (identified by its CIDR) to a Juju space. Removing a `juju_subnet` resource moves the subnet back to the default `alpha` space rather than deleting the underlying network.

## Reference an externally managed space

To reference a space that you've already created outside of the current Terraform plan, in your Terraform plan add a data source of the `juju_space` type, specifying the name of the space and the UUID of its hosting model. For example:

```terraform
data "juju_model" "my_model" {
  name = "default"
}

data "juju_space" "my_space_data_source" {
  model_uuid = data.juju_model.my_model.uuid
  name       = "my-space"
}
```

This is useful for failing early if the space does not exist, for example before referencing it in an endpoint binding.

> See more: [`juju_space` (data source)](../reference/terraform-provider/data-sources/space)

## Create a space

To create a space, in your Terraform plan add a resource of the `juju_space` type, specifying the name of the space and the UUID of the model. For example:

```terraform
resource "juju_model" "development" {
  name = "development"
}

resource "juju_space" "development" {
  model_uuid = juju_model.development.uuid
  name       = "development"
}
```

Changing the `name` forces replacement of the space.

> See more: [`juju_space` (resource)](../reference/terraform-provider/resources/space)

## Assign a subnet to a space

To assign a subnet to a space, in your Terraform plan add a resource of the `juju_subnet` type, specifying the model UUID, the CIDR of the subnet, and the target space name. The subnet must already exist in the cloud backing the model — the resource assigns it to the named space rather than creating it. For example:

```terraform
resource "juju_subnet" "development" {
  model_uuid = juju_model.development.uuid
  cidr       = "10.0.0.0/24"
  space_name = juju_space.development.name
}
```

Changing the `cidr` forces replacement. Changing the `space_name` moves the subnet to the new space.

> See more: [`juju_subnet` (resource)](../reference/terraform-provider/resources/subnet)

## List subnets

To list the subnets in a model, optionally filtered by space or availability zone, use the `juju_subnets` data source. For example:

```terraform
data "juju_subnets" "this" {
  model_uuid = juju_model.development.uuid

  # Optional: filter by space name
  # space_name = "alpha"

  # Optional: filter by availability zone
  # zone_name = "zone-1"
}

output "subnets_count" {
  value = length(data.juju_subnets.this.subnets)
}
```

> See more: [`juju_subnets` (data source)](../reference/terraform-provider/data-sources/subnets)

## List spaces

To list all spaces in a model, use the `list` block with the `juju_space` resource type. You can optionally filter by `name`. For example:

```terraform
list "juju_space" "this" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = juju_model.development.uuid

    # Optional: filter by space name
    # name = "my-space"
  }
}
```

> See more: [`juju_space` (list resource)](../reference/terraform-provider/list-resources/space)

## Remove a space or subnet assignment
> See also: {external+juju:ref}`Juju | Removing things <removing-things>`

To remove a subnet assignment, remove the `juju_subnet` resource definition from your Terraform plan. The subnet will be moved back to the default `alpha` space.

To remove a space, remove the `juju_space` resource definition from your Terraform plan. Any subnets still assigned to the space will be moved back to the `alpha` space automatically.

> See more: [`juju_space` (resource)](../reference/terraform-provider/resources/space), [`juju_subnet` (resource)](../reference/terraform-provider/resources/subnet)
