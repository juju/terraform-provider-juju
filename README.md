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

__Note:__ Commits provided as part of a PR must be [signed via git](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits) for the PR to merge.

### Planning & Design

When creating a new resource, datasource or changing a current schema please document the 
changes and create an github issue for review and approval before coding.

Consider writing documents for the project-docs/decisions folder.

New resources and datasources will require import and use example documents.

### Coding

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

See also [Building The Provider](#building-the-provider)

Prior to running the tests locally, ensure you have the following environmental variables set:

- `JUJU_CONTROLLER_ADDRESSES`
- `JUJU_USERNAME`
- `JUJU_PASSWORD`
- `JUJU_CA_CERT`

For example, here they are set using the currently active controller:

```shell
export CONTROLLER=$(juju whoami | yq .Controller)
export JUJU_CONTROLLER_ADDRESSES="$(juju show-controller | yq '.[$CONTROLLER]'.details.\"api-endpoints\" | tr -d "[]' "|tr -d '"'|tr -d '\n')"
export JUJU_USERNAME="$(cat ~/.local/share/juju/accounts.yaml | yq .controllers.$CONTROLLER.user|tr -d '"')"
export JUJU_PASSWORD="$(cat ~/.local/share/juju/accounts.yaml | yq .controllers.$CONTROLLER.password|tr -d '"')"
export JUJU_CA_CERT="$(juju show-controller $(echo $CONTROLLER|tr -d '"') | yq '.[$CONTROLLER]'.details.\"ca-cert\"|tr -d '"'|sed 's/\\n/\n/g')"
```

Then, finally, run the Acceptance tests:

```shell
make testlxd
```
And
```shell
make testmicrok8s
```
_Note:_ Acceptance tests create real resources.

### Staying in sync

To simplify staying in sync with upstream, give it a "remote" name:

```shell
git remote add upstream https://github.com/juju/terraform-provider-juju.git
```

Make sure your local copy and GitHub fork stay in sync with upstream:

```shell
git pull upstream/main --rebase
```

Merge commits for sync actions will be rejected.

### Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

### Debugging

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
