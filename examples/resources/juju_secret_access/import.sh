# Secret access can be imported by using the URI as in the juju show-secrets output.
# Example:
# $juju show-secret secret-name
# coh2uo2ji6m0ue9a7tj0:
#   revision: 1
#   owner: <model>
#   name: secret-name
#   created: 2024-04-19T08:46:25Z
#   updated: 2024-04-19T08:46:25Z
$ terraform import juju_access_secret.access-secret-name coh2uo2ji6m0ue9a7tj0