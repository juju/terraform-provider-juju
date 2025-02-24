(manage-clouds)=
# Manage clouds

> See also: {external+juju:ref}`Juju | Cloud <cloud>`

(add-a-kubernetes-cloud)=
## Add a Kubernetes cloud

To add a Kubernetes cloud to the controller that your Terraform plan is connected to, in your Terraform plan add a resource of the `juju_kubernetes_cloud` type, specifying a name and the path to the kubeconfig file. The example below does this and also creates a model associated with the new cloud:

```terraform
resource "juju_kubernetes_cloud" "my-k8s-cloud" {
  name              = "my-k8s-cloud"
  kubernetes_config = file("<path-to-my-kubennetes-cloud-config>.yaml")
}

resource "juju_model" "my-model" {
  name       = "my-model"
  credential = juju_kubernetes_cloud.my-k8s-cloud.credential
  cloud {
    name = juju_kubernetes_cloud.my-k8s-cloud.name
  }
}
```

> See more: [`juju_kubernetes_cloud`](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/kubernetes_cloud)

(manage-access-to-a-cloud)=
## Manage access to a cloud


```{note}
At present the Terraform Provider for Juju supports cloud access management only for clouds added to a Juju controller added to JIMM.
```

When using Juju with JAAS, to grant one or more users, groups, and/or service accounts access to a JAAS-known cloud, in your Terraform plan add a resource type `juju_jaas_access_cloud`, specifying the cloud name, the JAAS cloud access level, and the desired list of users, groups, and/or service accounts. For example:

```terraform
resource "juju_jaas_access_cloud" "development" {
  cloud_name       = "aws"
  access           = "can_addmodel"
  users            = ["foo@domain.com"]
  groups           = [juju_jaas_group.development.uuid]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
}
```

> See more: [`juju_jaas_access_cloud`](https://registry.terraform.io/providers/juju/juju/latest/docs/resources/jaas_access_cloud), [JAAS | Cloud access levels](https://canonical-jaas-documentation.readthedocs-hosted.com/en/latest/reference/authorisation_model/#cloud)