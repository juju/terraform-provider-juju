(manage-roles)=
# Manage roles

```{note}
In the Juju ecosystem, roles are supported only when using [JAAS](https://canonical-jaas-documentation.readthedocs-hosted.com/en/latest/).
```

(reference-an-externally-managed-role)=
## Reference an externally managed role

To reference a role you've created outside of the current Terraform plan, in your Terraform plan add a data source of the `juju_jaas_role` type, specifying the name of the role. Optionally, you may also output the role's UUID so you can later reference it in other resources. For example:

```terraform
data "juju_jaas_role" "test" {
  name = "role-0"
}
output "role_uuid" {
  value = data.juju_jaas_role.test.uuid
}
```

> See more: [`juju_jaas_role` (data source)](https://registry.terraform.io/providers/juju/juju/latest/docs/data-sources/jaas_role)

(add-a-role)=
## Add a role

To add a role, in your Terraform plan create a resource of the `juju_jaas_role` type, specifying its name. For example:

```terraform
resource "juju_jaas_role" "development" {
  name = "model-reader"
}
```

> See more: [`juju_jaas_role` (resource)](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/jaas_role)

(manage-access-to-a-role)=
## Manage access to a role

When using Juju with JAAS, to grant access to a role, in your Terraform plan add a resource type `juju_jaas_access_role`. Access can be granted to one or more users, service accounts, and/or groups. You must specify the role, the JAAS role access level, and the list of desired users, service accounts, and/or groups. For example:


```{note}
At present, the only valid JAAS role access level is `assignee`, so granting an entity access to a role effectively means giving them a particular role.
```


```terraform
resource "juju_jaas_access_role" "development" {
  role_id          = juju_jaas_role.target-role.uuid
  roles            = [juju_jaas_role.development.uuid]
  access           = "assignee"
  users            = ["foo@domain.com"]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
}
```

> See more: [`juju_jaas_access_role`](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/jaas_access_role), [JAAS | List of role relations](https://canonical-jaas-documentation.readthedocs-hosted.com/en/latest/reference/role/#list-of-role-relations)

## Manage a role's access to a controller, cloud, model, or offer

> See more: {ref}`manage-access-to-a-controller`, {ref}`manage-access-to-a-cloud`, {ref}`manage-access-to-a-model`, {ref}`manage-access-to-an-offer`