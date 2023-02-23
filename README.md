# Terraform Provider for Juju

> **Warning** The provider is under active development and will initially support only some Juju functionality. Use releases at your own risk.

The provider can be used to interact with Juju - a model-driven Operator Lifecycle Manager (OLM).

## Initial Scope

Once complete, the initial released version of the provider will allow you to:

- Manage models,
- Deploy charms from CharmHub,
- Manage integrations (previously named "relationships").
- Manage users
- Manage credentials
- Manage SSH keys

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 0.12.0
- [Go](https://golang.org/doc/install) >= 1.19

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

## Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

## Using the Provider

Please, refer to the [Terraform docs for the Juju provider](https://registry.terraform.io/providers/juju/juju/latest/docs).

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `go generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

_Note:_ Acceptance tests create real resources.

Prior to running the tests locally, ensure you have the following environmental variables set:

- `JUJU_CONTROLLER_ADDRESSES`
- `JUJU_USERNAME`
- `JUJU_PASSWORD`
- `JUJU_CA_CERT`

For example, here they are set using the currently active controller:

```shell
CONTROLLER=$(juju whoami | yq .Controller)
export JUJU_CONTROLLER_ADDRESSES="$(juju show-controller | yq '.['$CONTROLLER']'.details.\"api-endpoints\" | tr -d "[]' "|tr -d '"'|tr -d '\n')"
export JUJU_USERNAME="$(cat ~/.local/share/juju/accounts.yaml | yq .controllers.$CONTROLLER.user|tr -d '"')"
export JUJU_PASSWORD="$(cat ~/.local/share/juju/accounts.yaml | yq .controllers.$CONTROLLER.password|tr -d '"')"
export JUJU_CA_CERT="$(juju show-controller $(echo $CONTROLLER|tr -d '"') | yq '.['$CONTROLLER']'.details.\"ca-cert\"|tr -d '"'|sed 's/\\n/\n/g')"
```

Then, finally, run the tests:

```shell
make testacc
```

#### Linting

This repository uses [golangci-lint](https://golangci-lint.run/) as a linting tool as it can run multiple linters. The configuration for this tool is all handled in the file `.golangci.yaml` in the root of the repository allowing all runs of the tool to run with the same settings. When installed you can run the analysis with:

```shell
golangci-lint run
```

You can also integrate `golangci-lint` with some IDEs following instructions available here: [Editor integration](https://golangci-lint.run/usage/integrations)
