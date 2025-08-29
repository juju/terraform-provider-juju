(channel-revision)=
# Channel and Revision

The `channel` and `revision` fields can be set in your Terraform plan when deploying a Juju application.

Example:

```terraform
resource "juju_application" "this" {
    model = <model>

    charm {
        name     = "<charm-name>"
        channel  = "<channel-name>"
        revision = "<revision-number>"
    }
}
```

Both fields are optional. The following scenarios describe what happens when they are omitted:

#### Both Channel and Revision specified (recommended)

If both `channel` and `revision` are specified, the Terraform provider will deploy the requested revision from the requested channel.

#### Channel specified, Revision not specified

If only `channel` is specified, the provider will deploy the latest revision available in that channel at the time the application is *created*. The charm will not be refreshed on subsequent `terraform apply` runs.

#### Channel not specified, Revision specified

If only `revision` is specified, the provider will deploy that revision from the `stable` channel.

#### Neither Channel nor Revision specified

If neither field is specified, the provider will deploy the latest revision from the `stable` channel at the time the application is *created*. The charm will not be refreshed on subsequent `terraform apply` runs.

## Refreshing Charms

The Terraform provider does not support refreshing the charm when the revision is not specified.  
When unset, the revision number is determined during creation. If you wish to keep the revision unset, you can refresh the application manually using the Juju CLI.

## Recommendation

To ensure your environment is reproducible regardless of when the application is created, always set both `channel` and `revision`.
