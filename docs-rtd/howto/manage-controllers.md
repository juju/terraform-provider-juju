(manage-controllers)=
# Manage controllers


(manage-access-to-a-controller)=
## Manage access to a controller

```{note}
At present the Terraform Provider for Juju supports controller access management only for Juju controllers added to JIMM.
```

When using Juju with JAAS, to grant one or more users, groups, and/or service accounts access to a Juju controller added to JIMM, in your Terraform plan add a resource type `juju_jaas_access_controller`, specifying the model UUID, the JAAS controller access level, and the desired list of users, groups, and/or service accounts. For example:

```terraform
resource "juju_jaas_access_controller" "development" {
  access           = "administrator"
  users            = ["foo@domain.com"]
  groups           = [juju_jaas_group.development.uuid]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
}
```

> See more: [`juju_jaas_access_controller`](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/jaas_access_controller), [JAAS | Controller access levels](https://canonical-jaas-documentation.readthedocs-hosted.com/en/latest/reference/authorisation_model/#controller)