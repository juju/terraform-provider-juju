(manage-ssh-keys)=
# Manage SSH keys

> See also: {external+juju:ref}`Juju | SSH key <ssh-key>`

## Add an SSH key

To add a public `ssh` key to a model, in your Terraform plan create a resource of the `juju_ssh_key` type, specifying the name of the model and the payload (here, the SSH key itself). For example:

```text
resource "juju_ssh_key" "mykey" {
  model_uuid = juju_model.development.uuid
  payload    = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1I8QDP79MaHEIAlfh933zqcE8LyUt9doytF3YySBUDWippk8MAaKAJJtNb+Qsi+Kx/RsSY02VxMy9xRTp9d/Vr+U5BctKqhqf3ZkJdTIcy+z4hYpFS8A4bECJFHOnKIekIHD9glHkqzS5Vm6E4g/KMNkKylHKlDXOafhNZAiJ1ynxaZIuedrceFJNC47HnocQEtusPKpR09HGXXYhKMEubgF5tsTO4ks6pplMPvbdjxYcVOg4Wv0N/LJ4ffAucG9edMcKOTnKqZycqqZPE6KsTpSZMJi2Kl3mBrJE7JbR1YMlNwG6NlUIdIqVoTLZgLsTEkHqWi6OExykbVTqFuoWJJY3BmRAcP9H3FdLYbqcajfWshwvPM2AmYb8V3zBvzEKL1rpvG26fd3kGhk3Vu07qAUhHLMi3P0McEky4cLiEWgI7UyHFLI2yMRZgz23UUtxhRSkvCJagRlVG/s4yoylzBQJir8G3qmb36WjBXxpqAGHfLxw05EQI1JGV3ReYOs= user@somewhere"
}
```

> See more: [`juju_ssh_key` (resource)](../reference/terraform-provider/resources/ssh_key)

## Remove an SSH key

To remove an SSH key, remove its resource definition from your Terraform plan.

> See more: [`juju_ssh_key` (resource)](../reference/terraform-provider/resources/ssh_key)

