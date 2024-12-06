# Terraform Provider for Juju (`terraform-provider-juju`)

```{toctree}
:maxdepth: 2
:hidden: true

tutorial
howto/index
```

<!--
reference/index
explanation/index
-->

The Terraform Provider for Juju (henceforth, `terraform-provider-juju`) is a [Terraform Provider](https://developer.hashicorp.com/terraform/language/providers) that extends [Terraform](https://developer.hashicorp.com/terraform) with [Juju](https://juju.is) functionality.

When you're putting together your Terraform plan, if you specify `juju` as the provider, you can connect to a pre-existing Juju controller (created with the [`juju` CLI](https://juju.is/docs/juju/juju-client)) and then go ahead and use it to do Juju things -- easy deploy, configure, integrate, scale, etc., applications on any Juju-supported cloud (Kubernetes or otherwise) using [charms](https://juju.is/docs/juju/charmed-operator).

`terraform-provider-juju` combines the power of Terraform -- comprehensive infrastructure management, declaratively -- with the power of Juju -- easy systems management, from Day 0 to Day n.

Like all of Juju, `terraform-provider-juju` is for SREs, or anyone looking to take control of cloud. 

---------

## In this documentation

````{grid} 1 1 2 2

```{grid-item-card} [Tutorial](tutorial)
:link: tutorial
:link-type: doc

**Start here**: a hands-on introduction to Example Product for new users
```

```{grid-item-card} [How-to guides](/index)
:link: howto/index
:link-type: doc

**Step-by-step guides** covering key operations and common tasks
```

````

<!--
````{grid} 1 1 2 2
:reverse:

```{grid-item-card} [Reference](/index)
:link: reference/index
:link-type: doc

**Technical information** - specifications, APIs, architecture
```

```{grid-item-card} [Explanation](/index)
:link: explanation/index
:link-type: doc

**Discussion and clarification** of key topics
```

````
-->

---------


## Project and community

Example Project is a member of the Ubuntu family. It’s an open source project that warmly welcomes community projects, contributions, suggestions, fixes and constructive feedback.

* **[Read our code of conduct](https://ubuntu.com/community/ethos/code-of-conduct)**:
As a community we adhere to the Ubuntu code of conduct.

* **[Get support](https://discourse.charmhub.io/)**:
Discourse is the go-to forum for all questions Juju.

* **[Join our online chat](https://matrix.to/#/#terraform-provider-juju:ubuntu.com)**:
Meet us in the `#terraform-provider-juju` channel on Matrix.

* **[Report bugs](https://github.com/juju/terraform-provider-juju/issues/new?title=doc%3A+ADD+A+TITLE&body=DESCRIBE+THE+ISSUE%0A%0A---%0ADocument:%20index.md)**:
We want to know about the problems so we can fix them.

* **[Contribute docs](https://github.com/juju/terraform-provider-juju/tree/main/docs)**:
The documentation sources on GitHub.
