# Cross Controller Tests

This is how to setup the environment to run tests with cross controller relations.

## Setup offering controller.

```bash
juju bootstrap lxd offering-controller
juju add-model offering-model
juju deploy juju-qa-dummy-source
juju offer dummy-source:sink
```

## Bootstrap the main controller.

```bash
juju bootstrap lxd main
```

## Generate the env file

`make generate-env-file-with-offering-controller offering-controller`


## Run the tests

Run the tests using the env vars contained in the file generate by the command above.
