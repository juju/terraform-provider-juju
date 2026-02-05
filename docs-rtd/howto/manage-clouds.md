---
myst:
  html_meta:
    description: "Learn how to add machine and Kubernetes clouds to Juju controllers and manage cloud access permissions with JAAS."
---

(manage-clouds)=
# Manage clouds

> See also: {external+juju:ref}`Juju | Cloud <cloud>`


(add-a-machine-cloud)=
## Add a machine cloud
To add a machine cloud to the controller that your Terraform plan is connected to, in your Terraform plan add a resource of the `juju_cloud` type. For example:
```terraform
resource "juju_cloud" "this" {
  name = "my-cloud"
  type = "openstack"

  auth_types = ["userpass"]

  endpoint          = "https://cloud.example.com"
  identity_endpoint = "https://identity.example.com"
  storage_endpoint  = "https://storage.example.com"

  ca_certificates = [
    file("${path.module}/ca.pem"),
  ]

  regions = [
    {
      name              = "my-first-region"
      endpoint          = "https://region-default.example.com"
      identity_endpoint = "https://identity-default.example.com"
      storage_endpoint  = "https://storage-default.example.com"
    },
    {
      name = "my-other-region"
    },
  ]
}
```

Please note, in the list of regions, the first region is the default regions.

For further details on adding clouds to Juju, please read: [`add-cloud`](https://documentation.ubuntu.com/juju/3.6/reference/juju-cli/list-of-juju-cli-commands/add-cloud/).

(add-a-kubernetes-cloud)=
## Add a Kubernetes cloud

To add a Kubernetes cloud to the controller that your Terraform plan is connected to, in your Terraform plan add a resource of the `juju_kubernetes_cloud` type, specifying a name and the path to the kubeconfig file. The example below does this and also creates a model associated with the new cloud:

```terraform
resource "juju_kubernetes_cloud" "my-k8s-cloud" {
  name              = "my-k8s-cloud"
  kubernetes_config = file("<path-to-my-kubernetes-cloud-config>.yaml")
}

resource "juju_model" "my-model" {
  name       = "my-model"
  credential = juju_kubernetes_cloud.my-k8s-cloud.credential
  cloud {
    name = juju_kubernetes_cloud.my-k8s-cloud.name
  }
}
```

> See more: [`juju_kubernetes_cloud`](../reference/terraform-provider/resources/kubernetes_cloud)

(manage-access-to-a-cloud)=
## Manage access to a cloud


```{note}
At present the Terraform Provider for Juju supports cloud access management only for clouds added to a Juju controller added to JIMM.
```

When using Juju with JAAS, to grant access to a JAAS-known cloud, in your Terraform plan add a resource type `juju_jaas_access_cloud`. Access can be granted to one or more users, service accounts, roles, and/or groups. You must specify the cloud name, the JAAS cloud access level, and the desired list of users, service accounts, roles, and/or groups. For example:

```terraform
resource "juju_jaas_access_cloud" "development" {
  cloud_name       = "aws"
  access           = "can_addmodel"
  users            = ["foo@domain.com"]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
  roles            = [juju_jaas_role.development.uuid]
  groups           = [juju_jaas_group.development.uuid]
}
```

> See more: [`juju_jaas_access_cloud`](../reference/terraform-provider/resources/jaas_access_cloud), {external+jaas:ref}`JAAS | Cloud access levels <list-of-cloud-permissions>`
