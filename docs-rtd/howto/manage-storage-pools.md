---
myst:
  html_meta:
    description: "Learn how to create, reference, list, and remove storage pools in Juju models using the Terraform Provider for Juju."
---

(manage-storage-pools)=
# Manage storage pools

> See also: {external+juju:ref}`Juju | Storage <storage>`, {external+juju:ref}`Juju | Storage pool <storage-pool>`

## Reference an externally managed storage pool

To reference a storage pool that you've already created outside of the current Terraform plan, in your Terraform plan add a data source of the `juju_storage_pool` type, specifying the name of the storage pool and the UUID of its hosting model. For example:

```terraform
data "juju_model" "my_model" {
  name = "default"
}

data "juju_storage_pool" "my_storage_pool_data_source" {
  name       = "my-storage-pool"
  model_uuid = data.juju_model.my_model.uuid
}
```

> See more: [`juju_storage_pool` (data source)](../reference/terraform-provider/data-sources/storage_pool)

## Create a storage pool

To create a storage pool, in your Terraform plan add a resource of the `juju_storage_pool` type, specifying the name of the storage pool, the UUID of the model, and the storage provider type (for example, `tmpfs`, `rootfs`, `loop`, or a cloud-specific provider). You can optionally set `attributes` to pass provider-specific key-value pairs. For example:

```terraform
resource "juju_storage_pool" "mypool" {
  name             = "mypool"
  model_uuid       = juju_model.development.uuid
  storage_provider = "tmpfs"
  attributes = {
    a = "b"
    c = "d"
  }
}
```

> See more: [`juju_storage_pool` (resource)](../reference/terraform-provider/resources/storage_pool)

## Update a storage pool

To update a storage pool, in your Terraform plan, in the storage pool resource definition, change the `attributes` map. Applying the plan will update the storage pool in place. For example:

```terraform
resource "juju_storage_pool" "mypool" {
  name             = "mypool"
  model_uuid       = juju_model.development.uuid
  storage_provider = "tmpfs"
  attributes = {
    a = "updated-value"
  }
}
```

Changing the `name` or the `storage_provider` will force replacement of the storage pool.

> See more: [`juju_storage_pool` (resource)](../reference/terraform-provider/resources/storage_pool)

## List storage pools

To list all storage pools in a model, use the `list` block with the `juju_storage_pool` resource type. You can optionally filter by `name` or `storage_provider`. For example:

```terraform
list "juju_storage_pool" "this" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = juju_model.development.uuid

    # Optional: filter by storage pool name
    # name = "my-storage-pool"

    # Optional: filter by storage provider
    # storage_provider = "ebs"
  }
}
```

> See more: [`juju_storage_pool` (list resource)](../reference/terraform-provider/list-resources/storage_pool)

## Remove a storage pool
> See also: {external+juju:ref}`Juju | Removing things <removing-things>`

To remove a storage pool, remove its resource definition from your Terraform plan.

> See more: [`juju_storage_pool` (resource)](../reference/terraform-provider/resources/storage_pool)
