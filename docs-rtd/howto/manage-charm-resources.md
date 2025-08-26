(manage-charm-resources)=
# Manage charm resources

> See also: {external+juju:ref}`Juju | Resource (charm) <charm-resource>`

When you deploy / update an application from a charm, that automatically deploys / updates any charm resources, using the defaults specified by the charm author. However, you can also specify resources manually (e.g., to try a resource released only to `edge` or to specify a non-Charmhub resource). This document shows you how.

## Specify the resources to be deployed with a charm


To specify the resource(s) to be deployed with your charm, in your Terraform plan, in the definition of the resource for the application specify a `resources` block with key-value pairs listing resource names and their revision number. For example:

```terraform
resource "juju_application" "application_one" {
  name = "my-application"
  model = juju_model.testmodel.name

  charm {
    name = "juju-qa-test"
    channel = "2.0/edge"
  }
  resources = {
    "foo-file" = 4
  }
}
```


```{tip}

About `charm > revision` and `resources`:
- If you specify only `charm > revision`: This is equivalent to `juju deploy <charm> --revision` or `juju refresh <charm> --revision` -- that is, the resource revision is automatically the latest.
- If you specify only `resources`: This is equivalent to `juju attach-resource` -- that is, the resource revision is whatever you've specified.

**Note:** While `juju refresh <charm> --resource` allows you to update a resource even if no update is available for the charm, this is not possible with `terraform juju`.

```

> See more: [`juju_application > resources`](../reference/terraform-provider//resources/application)
