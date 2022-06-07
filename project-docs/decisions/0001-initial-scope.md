# Initial Scope of the Juju Terraform Provider

## Context and Problem Statement

The initial development scope of the provider needs to be constrained because it is not feasible to target all of Juju's capabilities.  

## Decision

The initial scope is to focus on the development of a Terraform Provider which allows the management of:

- Models,
- Charms from CharmHub,
- Relationships.

Implicitly there will be support for importing these resource types into existing Terraform projects.

There is also a desire to support relationships between Models, however, this is considered a stretch-goal.

This represents an ["MVP"][0] Juju Terraform provider.

Charms available from Charm Store and locally are specifically out of scope. Other capabilities of Juju are out of scope.

[0]: https://en.wikipedia.org/wiki/Minimum_viable_product
