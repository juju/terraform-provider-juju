# Options for Controller Authentication

## Context and Problem Statement

The provider should support existing authentication methods i.e.:

- username and password,
- macaroons,
- credentials within `~/.local/share/juju`

The most appropriate authentication mechanism should be selected based on provider configuration. This adds additional complexity for the MVP. 

## Decision

Username and password authentication will be prioritised for the MVP, other authentication mechanisms can be added later.

## References

- [Juju - How to manage credentials][0]

[0]: https://juju.is/docs/olm/manage-credentials
