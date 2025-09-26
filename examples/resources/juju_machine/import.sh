# Machines can be imported using the format: `model_uuid:machine_id:machine_name`.
# The value of machine_id is the Juju Machine ID. machine_name is an optional 
# name you can define in Terraform for the machine. It is not used in Juju.
# Here is an example to import a machine from a model with machine ID 1 and a 
# name "machine_one":
$ terraform import juju_machine.machine_one 4ffb2226-6ced-458b-8b38-5143ca190f75:1:machine_one
