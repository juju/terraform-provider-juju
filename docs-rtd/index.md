# Terraform Provider for Juju

```{toctree}
:maxdepth: 2
:hidden: true

tutorial
howto/index
Reference <reference/index>
```

<!--
reference/index
explanation/index
-->

The Terraform Provider for Juju is a [Terraform Provider](https://developer.hashicorp.com/terraform/language/providers) that extends [Terraform](https://developer.hashicorp.com/terraform) with [Juju](https://documentation.ubuntu.com/juju) and [JAAS](https://jaas.ai/) functionality.

When you're putting together your Terraform plan, if you specify `juju` as the provider, you can connect to a pre-existing Juju controller or JIMM controller and then go ahead and use it to do Juju things -- easy deploy, configure, integrate, scale, etc., infrastructure and applications on any Juju-supported cloud (Kubernetes or otherwise) using charms.

The Terraform Provider for Juju combines the power of Terraform -- comprehensive infrastructure management, declaratively -- with the power of Juju -- easy systems management, from Day 0 to Day n.

Like all of Juju, the Terraform Provider for Juju is for SREs, or anyone looking to take control of cloud.

---------

## In this documentation

- **Set up the Terraform Provider for Juju:** {ref}`Install <install-the-terraform-provider-for-juju>`, {ref}`Connect a controller <manage-controllers>`, {ref}`Connect a cloud <manage-clouds>`, {ref}`Add a model <manage-models>`
- **Handle authorization:** {ref}`SSH keys <manage-ssh-keys>`, {ref}`Users <manage-users>`, {ref}`Service accounts <manage-service-accounts>`, {ref}`Roles <manage-roles>`, {ref}`Groups <manage-groups>`
- **Deploy infrastructure and applications:** {ref}`Deploy <deploy-an-application>`, {ref}`Configure <configure-an-application>`, {ref}`Integrate <integrate-an-application-with-another-application>`, {ref}`Scale <scale-an-application>`, {ref}`Upgrade <upgrade-an-application>`, etc.

```{grid-item-card} [Tutorial](tutorial)
:link: tutorial
:link-type: doc

**Start here**: a hands-on introduction to the Terraform Provider for Juju for new users <br>
```

```{grid-item-card} [How-to guides](/index)
:link: howto/index
:link-type: doc

**Step-by-step guides** covering key operations and common tasks
```

```{grid-item-card} [Reference](/index)
:link: reference/index
:link-type: doc

**Technical information** - specifications, APIs, architecture
```

---------

<!-- {ref}`tutorial-plan` | {ref}`tutorial-deploy-configure-integrate` | {ref}`tutorial-scale` -->


## Project and community

The Terraform Provider for Juju is a member of the Ubuntu family. It’s an open source project that warmly welcomes community projects, contributions, suggestions, fixes and constructive feedback.

* [Release notes](https://github.com/juju/terraform-provider-juju/releases )

* [Code of conduct](https://ubuntu.com/community/ethos/code-of-conduct)

* [Join our chat](https://matrix.to/#/#terraform-provider-juju:ubuntu.com)

* [Join our forum](https://discourse.charmhub.io/)

* [Report a bug](https://github.com/juju/terraform-provider-juju/issues/new?title=doc%3A+ADD+A+TITLE&body=DESCRIBE+THE+ISSUE%0A%0A---%0ADocument:%20index.md)

* [Contribute](https://github.com/juju/terraform-provider-juju/blob/main/CONTRIBUTING.md)

* [Visit our careers page](https://juju.is/careers)

Thinking about using Juju for your next project? [Get in touch!](https://canonical.com/contact-us)
