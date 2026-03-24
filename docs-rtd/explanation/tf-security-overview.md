---
myst:
  html_meta:
    description: "Comprehensive Juju Terraform security overview."
---

(tf-security-overview)=
# Security overview

This document provides an overview of security hardening considerations for the
Juju Terraform Provider, focusing on state security, communication, and general
Terraform best practices.

## Provider scope

The Juju Terraform Provider is a **client-only** tool — it does not run any
server-side processes or store data of its own. All state about managed
infrastructure is handled by Terraform itself and stored in the Terraform state
file.

Because the provider is a client, the primary security surface areas to consider
are:

- The **Terraform state file**, which may contain sensitive information.
- The **credentials** used to authenticate with the Juju controller.
- The **network communication** between the provider and the Juju controller.

## Terraform state security

The Terraform state file is a critical security boundary. The Juju Terraform
Provider manages resources that can store sensitive data in state, including:

- `juju_credential` — cloud credentials used by Juju to provision
  infrastructure.
- `juju_secret` — user-defined secrets managed by Juju.
- Any resource attributes that Terraform marks as sensitive.

```{caution}
Anyone with access to the Terraform state file can read these values in plain
text. Treat the state file with the same care as a secrets store.
```

To protect the state file:

- **Use remote state backends** with access controls, such as S3 with bucket
  policies, for example: [s3 backend Terraform doc](https://developer.hashicorp.com/terraform/language/backend/s3). 
  Avoid committing state files to version control.
- **Restrict access** to the backend storage using least-privilege IAM policies
  or equivalent access controls. Only CI systems or automation pipelines should
  have write access.
- **Encrypt state at rest** by enabling server-side encryption on the chosen
  remote backend.

## Communication security

### Provider — Juju controller

The Juju Terraform Provider communicates with the Juju controller exclusively
over **TLS-encrypted connections**. This ensures that credentials, resource
configuration, and other data exchanged during `terraform plan` and
`terraform apply` are protected in transit.

The minimum TLS version enforced by the Juju client libraries is **TLS v1.2**.

No plain-text communication paths are used between the provider and the
controller.

### Authentication

The provider authenticates with the Juju controller using credentials supplied
via provider configuration or environment variables. When used with JAAS, the
client credential (machine-to-machine) OAuth 2.0 flow is used, as described in
the [JAAS security overview](https://documentation.ubuntu.com/jaas/v3/explanation/jaas-security/).

To reduce credential exposure:

- Pass credentials through environment variables or a secrets manager rather
  than hard-coding them in `.tf` files.
- Apply least-privilege access to the Juju user or service account used by the
  provider.

## General Terraform hardening

Because the Juju Terraform Provider is a standard Terraform provider, all
general Terraform security practices apply. HashiCorp's
[Terraform security: 5 foundational practices](https://www.hashicorp.com/en/blog/terraform-security-5-foundational-practices)
is an excellent starting point.

These practices apply equally to all Terraform providers, including the Juju
Terraform Provider.
