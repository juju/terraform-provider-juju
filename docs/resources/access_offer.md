---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "juju_access_offer Resource - terraform-provider-juju"
subcategory: ""
description: |-
  A resource that represent a Juju Access Offer. Warning: Do not repeat users across different access levels.
---

# juju_access_offer (Resource)

A resource that represent a Juju Access Offer. Warning: Do not repeat users across different access levels.

## Example Usage

```terraform
resource "juju_access_offer" "this" {
  offer_url = juju_offer.my_application_offer.url
  consume   = [juju_user.dev.name]
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `offer_url` (String) The url of the offer for access management. If this is changed the resource will be deleted and a new resource will be created.

### Optional

- `admin` (Set of String) List of users to grant admin access. "admin" user is not allowed.
- `consume` (Set of String) List of users to grant consume access. "admin" user is not allowed.
- `read` (Set of String) List of users to grant read access. "admin" user is not allowed.

### Read-Only

- `id` (String) The ID of this resource.

## Import

Import is supported using the following syntax:

```shell
# Access Offers can be imported by using the Offer URL as in the juju show-offers output.
# Example:
# $juju show-offer mysql
# Store            URL             Access  Description                                    Endpoint  Interface  Role
# mycontroller     admin/db.mysql  admin   MariaDB Server is one of the most ...          mysql     mysql      provider
$ terraform import juju_access_offer.db admin/db.mysql
```
