---
myst:
  html_meta:
    description: "Terraform Provider for Juju extends Terraform with Juju functionality to deploy, configure, and manage infrastructure and applications on any cloud."
relatedlinks: "[Charmcraft](https://documentation.ubuntu.com/charmcraft/), [Charmlibs](https://canonical-charmlibs.readthedocs-hosted.com/), [Concierge](https://github.com/canonical/concierge), [JAAS](https://documentation.ubuntu.com/jaas/), [Jubilant](https://documentation.ubuntu.com/jubilant/), [Juju](https://documentation.ubuntu.com/juju/), [Ops](https://documentation.ubuntu.com/ops/), [Pebble](https://documentation.ubuntu.com/pebble/)"
---

(home)=
# Terraform Provider for Juju documentation

```{toctree}
:maxdepth: 2
:hidden: true

Tutorial <tutorial>
How-to guides <howto/index>
Reference <reference/index>
Explanation <explanation/index>
```

The Terraform Provider for Juju is a [Terraform Provider](https://developer.hashicorp.com/terraform/language/providers) that extends [Terraform](https://developer.hashicorp.com/terraform) with [Juju](https://documentation.ubuntu.com/juju) and [JAAS](https://documentation.ubuntu.com/jaas) functionality.

When you're putting together your Terraform plan, if you specify `juju` as the provider, you can connect to a pre-existing Juju controller or JIMM controller and then go ahead and use it to do Juju things -- easy deploy, configure, integrate, scale, etc., infrastructure and applications on any Juju-supported cloud (Kubernetes or otherwise) using charms.

The Terraform Provider for Juju combines the power of Terraform -- comprehensive infrastructure management, declaratively -- with the power of Juju -- easy systems management, from Day 0 to Day n.

Like all of Juju, the Terraform Provider for Juju is for SREs, or anyone looking to take control of cloud.

## In this documentation

- **Set up the Terraform Provider for Juju:** {ref}`Install <install-the-terraform-provider-for-juju>` • {ref}`Connect a controller <manage-controllers>` • {ref}`Connect a cloud <manage-clouds>`
- **Handle authentication and authorization:** {ref}`Manage users <manage-users>` • {ref}`Manage service accounts <manage-service-accounts>`
- **Deploy infrastructure and applications:** {ref}`Deploy <deploy-an-application>` • {ref}`Configure <configure-an-application>` • {ref}`Integrate <integrate-an-application-with-another-application>` • {ref}`Scale <scale-an-application>` • {ref}`Upgrade <upgrade-an-application>`

## How this documentation is organised

This documentation uses the [Diátaxis documentation structure](https://diataxis.fr/).

- The {ref}`Tutorial <tutorial>` takes you step-by-step through using the provider to deploy an application.
- {ref}`How-to guides <howtos>` assume you have basic familiarity with Terraform and Juju.
- {ref}`Reference <reference>` provides technical specifications for provider resources and data sources.
- {ref}`Explanation <explanation>` includes security models and integration patterns.


## Project and community

The Terraform Provider for Juju is a member of the Ubuntu family. It's an open source project that warmly welcomes community contributions, suggestions, fixes and constructive feedback.

### Get involved

* [Join our chat](https://matrix.to/#/#terraform-provider-juju:ubuntu.com)
* [Join our forum](https://discourse.charmhub.io/)
* [Report a bug](https://github.com/juju/terraform-provider-juju/issues/new?title=doc%3A+ADD+A+TITLE&body=DESCRIBE+THE+ISSUE%0A%0A---%0ADocument:%20index.md)
* [Contribute](https://github.com/juju/terraform-provider-juju/blob/main/CONTRIBUTING.md)
* [Visit our careers page](https://canonical.com/careers/engineering)

### Releases

* [Release notes](https://github.com/juju/terraform-provider-juju/releases)

### Governance and policies

* [Code of conduct](https://ubuntu.com/community/ethos/code-of-conduct)

### Commercial support

Thinking about using the Terraform Provider for Juju for your next project? [Get in touch!](https://canonical.com/contact-us)
