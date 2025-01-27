(manage-service-accounts)=
# Manage service accounts

```{note}
In the Juju ecosystem, service accounts are supported only when using [JAAS](https://canonical-jaas-documentation.readthedocs-hosted.com/en/latest/).
```


(manage-access-to-a-service account)=
## Manage access to a service account

When using Juju with JAAS, to grant a user, a group, or a service account access to a JAAS controller, in your Terraform plan add a resource type `juju_jaas_access_controller`, specifying the model UUID, the JAAS controller access level, and the list of desired users, groups, and/or service accounts. For example:

```terraform
resource "juju_jaas_access_controller" "development" {
  access           = "administrator"
  users            = ["foo@domain.com"]
  groups           = [juju_jaas_group.development.uuid]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
}
```

> See more: [`juju_jaas_access_service_account`](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/jaas_access_service_account), [JAAS | Service account access levels](https://canonical-jaas-documentation.readthedocs-hosted.com/en/latest/reference/authorisation_model/#service-account)