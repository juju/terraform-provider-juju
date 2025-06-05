(manage-service-accounts)=
# Manage service accounts

```{note}
In the Juju ecosystem, service accounts are supported only when using [JAAS](https://canonical-jaas-documentation.readthedocs-hosted.com/en/latest/).
```


(manage-access-to-a-service account)=
## Manage access to a service account

When using Juju with JAAS, to grant access to a Juju controller connected to JIMM, in your Terraform plan add a resource type `juju_jaas_access_controller`. Access can be granted to one or more users, service accounts, roles, and/or groups. You must specify the model UUID, the JAAS controller access level, and the list of desired users, service accounts, roles, and/or groups. For example:

```terraform
resource "juju_jaas_access_controller" "development" {
  access           = "administrator"
  users            = ["foo@domain.com"]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
  roles            = [juju_jaas_role.development.uuid]
  groups           = [juju_jaas_group.development.uuid]
}
```

> See more: [`juju_jaas_access_service_account`](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/jaas_access_service_account), {external+jaas:ref}`JAAS | Service account access levels <list-of-service-account-permissions>`

## Manage a service account's access to a controller, cloud, model, offer, role, or group

> See more: {ref}`manage-access-to-a-controller`, {ref}`manage-access-to-a-cloud`, {ref}`manage-access-to-a-model`, {ref}`manage-access-to-an-offer`, {ref}`manage-access-to-a-role`, {ref}`manage-access-to-a-group`