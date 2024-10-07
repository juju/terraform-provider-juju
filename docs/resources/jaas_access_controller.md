---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "juju_jaas_access_controller Resource - terraform-provider-juju"
subcategory: ""
description: |-
  A resource that represents direct access the JAAS controller.
---

# juju_jaas_access_controller (Resource)

A resource that represents direct access the JAAS controller.

## Example Usage

```terraform
resource "juju_jaas_access_controller" "development" {
  access           = "administrator"
  users            = ["foo@domain.com"]
  groups           = [juju_jaas_group.development.uuid]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `access` (String) Level of access to grant. Changing this value will replace the Terraform resource. Valid access levels are described at https://canonical-jaas-documentation.readthedocs-hosted.com/en/latest/reference/authorisation_model/#valid-relations

### Optional

- `groups` (Set of String) List of groups to grant access.
- `service_accounts` (Set of String) List of service accounts to grant access.
- `users` (Set of String) List of users to grant access.

### Read-Only

- `id` (String) The ID of this resource.

## Import

Import is supported using the following syntax:

```shell
# JAAS controller access can be imported using the fixed JAAS controller name and access level
# I.e. in this case jimm is the only valid controller name.
$ terraform import juju_jaas_access_cloud.development jimm:administrator
```