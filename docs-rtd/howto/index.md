---
myst:
  html_meta:
    description: "Step-by-step guides for managing Terraform Provider for Juju: setup, authentication, deployment, and infrastructure operations."
---

(howtos)=
# How-to guides

**Step-by-step guides** covering key operations and common tasks

```{toctree}
:maxdepth: 2
:hidden:

create-deployment-dependencies
manage-the-terraform-provider-for-juju
manage-controllers
manage-clouds
manage-credentials
manage-models
manage-ssh-keys
manage-users
manage-service-accounts
manage-roles
manage-groups
manage-charms
manage-charm-resources
manage-applications
manage-relations
manage-offers
manage-units
manage-secrets
manage-machines
use-the-juju-cli
```

## Set up the Terraform Provider for Juju

Install the client, connect a Juju controller, connect clouds, add models.

- {ref}`manage-the-terraform-provider-for-juju`
- {ref}`manage-controllers`
- {ref}`manage-clouds`
- {ref}`manage-credentials`
- {ref}`manage-models`

## Handle authentication and authorization

Set up SSH keys. Add users, service accounts, roles, and groups and control their access to controllers, clouds, models, or application offers.

- {ref}`manage-ssh-keys`
- {ref}`manage-users`
- {ref}`manage-service-accounts`
- {ref}`manage-roles`
- {ref}`manage-groups`

## Deploy infrastructure and applications

Deploy, configure, integrate, scale, etc., charmed applications. This will automatically provision infrastructure, but you can customise it before, during, or after deploy too.

- {ref}`create-deployment-dependencies`
- {ref}`manage-charms`
- {ref}`manage-charm-resources`
- {ref}`manage-applications`
- {ref}`manage-relations`
- {ref}`manage-offers`
- {ref}`manage-units`
- {ref}`manage-secrets`
- {ref}`manage-machines`
- {ref}`use-the-juju-cli-in-terraform`
- {ref}`use-the-bootstrapped-controller`
