# Manually Provisioning Machines via SSH

## Context and Problem Statement

The current interface for the provider to support manual provisioning via 
ssh is through the `ssh_address`, `public_key_file` and `private_key_file` 
directives. However, this is not exactly compliant with the general 
conventions used in the terraform ecosystem, as for instance `$(file_path)` is 
widely used, and also loading it from a creds db (like a secrets store) is very 
common, e.g. `local.db_creds.private_key`.

The problem is that the Juju API (in particular, the `sshprovisiner.
ProvisionMachine`) doesn't accept a key string, only a file path), so doing 
it with actual keys would require awkward workarounds (or a change in the 
Juju API).

A side issue here is what to do when `constraints` or `series` is provided 
along with manual provisioning. Obviously we can't (trivially) dynamically 
establish new constraints or change the series on an existing machine.

## Decision

The decision is to keep it a file path for now, mostly to provide this 
functionality to the people as soon as possible. The ideal solution is 
probably to add the capability to accept keys directly into Juju's ssh 
provisioner (which might also have some security repercussions to be 
discussed). 

Also the `public_key_file` and the `private_key_file` are made to be required
arguments to the `ssh_address` directive, in order to save the provider from
dealing with loading the default juju client keys. This will probably be
resolved when/if we have separate client keys for different clients of juju
(e.g. juju cli, pylibjuju, terraform provider etc.). Currently on the
terraform provider, any manual provision will require ssh keys to be inputted.

Another consideration in doing it this way is that it ties the provider very
tightly to the Juju packages, as we're importing a bunch of packages
including but not limited to `cmd`, `ssh`, `sshprovisioner`,
`environs/manual`, along with a bunch of indirect imports. This is something
that we're trying to avoid, as we're trying to thin out the clients as much
as possible. Again, doing that would require implementing all that
functionality within the provider, which is in an of itself is not a problem.
However, a good balance in the API between Juju and the clients needs to be
established so that we wouldn't need to develop, maintain (and possibly
update) these on every client (currently the terraform provider, juju cli
and python-libjuju).

Also the decision on what to do when the `constraints` or `series` is 
provided along with manual provisioning is to error. So the scenario is 
that we provide the ssh_address of a machine that we want to manually 
provision, along with some `constraints` (e.g. mem=8G) or `series`. Normally 
this is something that juju cli ignores. However, based on discussions with 
@tiradojm, we'd like avoid having the user under any assumptions that these 
constraints and series will be used in the addressed machine, so we error 
instead of ignoring input.