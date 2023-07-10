.PHONY: help
help:
	@echo "Usage: \n"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | sort | column -t -s ':' |  sed -e 's/^/ /'

.PHONY: go-install
go-install:
## go-install: Build and install terraform-provider-juju in $GOPATH/bin
	@echo "Installing terraform-provider-juju"
	@go mod tidy
	@go install

GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
GOPATH=$(shell go env GOPATH)
VERSION=0.8.0
REGISTRY_DIR=~/.terraform.d/plugins/registry.terraform.io/juju/juju/${VERSION}/${GOOS}_${GOARCH}

.PHONY: install
install: simplify docs go-install
## install: Build terraform-provider-juju and copy to ~/.terraform.d using VERSION
	@echo "Copied to ~/.terraform.d/plugins/registry.terraform.io/juju/juju/${VERSION}/${GOOS}_${GOARCH}"
	@mkdir -p ${REGISTRY_DIR}
	@cp ${GOPATH}/bin/terraform-provider-juju ${REGISTRY_DIR}/terraform-provider-juju_v${VERSION}

.PHONY: simplify
# Reformat and simplify source files.
simplify:
## simplify: Format and simplify the go source code
	@echo "Formating the go source code"
	@gofmt -w -l -s .

.PHONY: lint
lint:
## lint: run the go linter
	@echo "Running go lint"
	@golangci-lint run -c .golangci.yml

HAS_TERRAFORM := $(shell command -v terraform 2> /dev/null)
.PHONY: docs
docs:
## docs: update the generated terraform docs.
ifneq ($(HAS_TERRAFORM),)
	@echo "Generating docs"
	@go generate ./...
else
	@echo "Unable to generate docs, terraform not installed"
endif

JUJU=juju
CONTROLLER=$(shell ${JUJU} whoami | yq .Controller)
CONTROLLER_ADDRESSES="$(shell ${JUJU} show-controller | yq .${CONTROLLER}.details.\"api-endpoints\" | tr -d "[]' "|tr -d '"'|tr -d '\n')"
USERNAME="$(shell cat ~/.local/share/juju/accounts.yaml | yq .controllers.${CONTROLLER}.user|tr -d '"')"
PASSWORD="$(shell cat ~/.local/share/juju/accounts.yaml | yq .controllers.${CONTROLLER}.password|tr -d '"')"
CA_CERT="$(shell ${JUJU} show-controller $(echo ${CONTROLLER}|tr -d '"')| yq .${CONTROLLER}.details.\"ca-cert\"|tr -d '"'|sed 's/\\n/\n/g')"

.PHONY: envtestlxd
envtestlxd:
## envtestlxd: Under development - Include env var and run unit tests against lxd
	JUJU_CONTROLLER_ADDRESSES=${CONTROLLER_ADDRESSES} \
	JUJU_USERNAME=${USERNAME} JUJU_PASSWORD=${PASSWORD} \
	JUJU_CA_CERT=${CA_CERT} TF_ACC=1 TEST_CLOUD=lxd go test ./... -v $(TESTARGS) -timeout 120m

.PHONY: testlxd
testlxd:
## testlxd: Run unit tests against lxd
	TF_ACC=1 TEST_CLOUD=lxd go test ./... -v $(TESTARGS) -timeout 120m

.PHONY: testmicrok8s
testmicrok8s:
## testmicrok8s: Run unit tests against microk8s
	TF_ACC=1 TEST_CLOUD=microk8s go test ./... -v $(TESTARGS) -timeout 120m
