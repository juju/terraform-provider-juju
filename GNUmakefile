default: testlxd

.PHONY: lint
lint:
	golangci-lint run -c .golangci.yml
  
.PHONY: testlxd
testlxd:
	TF_ACC=1 TEST_CLOUD=lxd go test ./... -v $(TESTARGS) -timeout 120m

.PHONY: testmicrok8s
testmicrok8s:
	TF_ACC=1 TEST_CLOUD=microk8s go test ./... -v $(TESTARGS) -timeout 120m
