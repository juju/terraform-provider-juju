# A minimal example of a juju_cloud resource.
# These are the only required fields.
resource "juju_cloud" "this" {
  name       = "my-cloud"
  type       = "openstack"
  auth_types = ["userpass"]
}

# A full example of all fields possible to be filled in a juju_cloud resource.
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

  # Note, the first region is the DEFAULT region.
  regions = [
    {
      name              = "default"
      endpoint          = "https://region-default.example.com"
      identity_endpoint = "https://identity-default.example.com"
      storage_endpoint  = "https://storage-default.example.com"
    },
    {
      name = "us-east-1"
    },
  ]
}