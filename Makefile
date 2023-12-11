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
EDGEVERSION=0.11.0
REGISTRY_DIR=~/.terraform.d/plugins/registry.terraform.io/juju/juju/${EDGEVERSION}/${GOOS}_${GOARCH}

.PHONY: install
install: simplify docs go-install
## install: Build terraform-provider-juju and copy to ~/.terraform.d using EDGEVERSION
	@echo "Copied to ~/.terraform.d/plugins/registry.terraform.io/juju/juju/${EDGEVERSION}/${GOOS}_${GOARCH}"
	@mkdir -p ${REGISTRY_DIR}
	@cp ${GOPATH}/bin/terraform-provider-juju ${REGISTRY_DIR}/terraform-provider-juju_v${EDGEVERSION}

.PHONY: simplify
# Reformat and simplify source files.
simplify:
## simplify: Format and simplify the go source code
	@echo "Formatting the go source code"
	@gofmt -w -l -s .

.PHONY: lint
lint:
## lint: run the go linter
	@echo "Running go lint"
	@golangci-lint run -c .golangci.yml

.PHONY: static-analysis
static-analysis:
## static-analysis: Check the go code using static-analysis
	@./tools/static-analysis.sh

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

PACKAGES=terraform golangci-lint go
# Function to check if Snap packages are installed
check-snap-package:
	@for package in $(PACKAGES); do \
		if snap list $$package >/dev/null 2>&1; then \
			echo "Package $$package is already installed."; \
		else \
			echo "Package $$package is not installed."; \
			sudo snap install $$package --classic; \
		fi; \
	done

.PHONY: install-snap-dependencies
install-snap-dependencies: check-snap-package
## install-snap-dependencies: Install all the snap dependencies

.PHONY: install-dependencies
install-dependencies: install-snap-dependencies
## install-dependencies: Install all the dependencies



