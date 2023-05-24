# CI variables

## Context and Problem Statement

In order to run our integration tests, we need to setup a controller on a given platform. We usually setup an lxd controller and run all our tests there. This somehow limits the scope of tests we can run and eliminates from our tests potential inconsistencies and/or features enabled in K8s. For example, in the case of setting OCI images we need to have a K8s environment.

At this moment, we use the `TF_ACC` environment variable to clearly state that we're running acceptance tests. This permits to skip those tests that require resources to be created.

## Decision

In order to enable tests to decide whether they have to run or not, we will use the `t.Skip()` function from the `testing` package. Those testing functions requiring specific setups will have to check whether this is up by checking the corresponding environment variables.

Test functions can check the following environment variables:

| Variable     | Values              | Description                                       |
| ------------ | ------------------- | ------------------------------------------------- |
| `TEST_CLOUD` | `lxd` or `microk8s` | Cloud available during testingLXD cloud available |
