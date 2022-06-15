# Terraform Provider for Juju

The provider can be used to interact with Juju - a model-driven Operator Lifecycle Manager (OLM).

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 0.12.0
- [Go](https://golang.org/doc/install) >= 1.18

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

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `go generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources.

Prior to running the tests locally, ensure you have the following environmental variables set:

* `JUJU_CONTROLLER`
* `JUJU_USERNAME`
* `JUJU_PASSWORD`
* `JUJU_CA_CERT`

For example, here they are set using a controller named `overlord`:

```shell
export JUJU_CONTROLLER="127.0.0.1:17070"
export JUJU_USERNAME="$(cat ~/.local/share/juju/accounts.yaml | yq .controllers.overlord.user)"
export JUJU_PASSWORD="$(cat ~/.local/share/juju/accounts.yaml | yq .controllers.overlord.password)"
export JUJU_CA_CERT="$(juju show-controller overlord | yq .overlord.details.ca-cert)"
```

Then, finally, run the tests: 
```shell
make testacc
```
