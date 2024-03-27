# Terraform Provider for Juju

> **Warning** The provider is under active development and will initially support only some Juju functionality. Use releases at your own risk.

The provider can be used to interact with Juju - an open source orchestration engine.

##  Scope

Today this provider allows you to manage the following via resources:

* Applications and deploy charms
* Credentials for existing clouds
* Integrations
* Machines
* Models
* Model ssh keys
* Offers
* Users

and refer to the following via data sources:

* Machines
* Models
* Offers

_Note:_ These features may not have functional parity with the juju CLI at this time.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.6
- [Go](https://golang.org/doc/install) >= 1.21

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider dependencies using the make `install-dependencies` target:

    ```shell
    make install-dependencies
    ```

1. Build the provider using the make `go-install` target:

    ```shell
    make go-install
    ```

1. Install in ~/.terraform.d/plugins with

    ```shell
    make install
    ```

Please run `make` to see other targets available, including formatting, linting and static analysis.


## Using the Provider

Please, refer to the [Terraform docs for the Juju provider](https://registry.terraform.io/providers/juju/juju/latest/docs).

## Developing the Provider

Please see the [Developing wiki](https://github.com/juju/terraform-provider-juju/wiki/Developing)

## Debugging

To debug, setup environment variables:

```shell
export TF_LOG_PROVIDER=TRACE ; export TF_LOG_PATH=./terraform.log
```

Run your terraform commands.

To find logs specific to the juju provider code:
```shell
grep "@module=juju.resource" ./terraform.log
grep "@module=juju.datasource" ./terraform.log
```

To find logs specific to the juju client talking to juju itself:
```shell
grep "@module=juju.client" ./terraform.log
```