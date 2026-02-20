# Bootstrap Tests

This is how to setup the environment to run tests to bootstrap controllers.

## LXD

`./tools/create_lxd_config.sh` will create `~/lxd-credentials.yaml` needed for the bootstrap tests.

## Microk8s

`sudo microk8s.config > ~/microk8s-config.yaml` will create the file needed for bootstrap tests.
