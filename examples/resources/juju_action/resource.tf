resource "juju_action" "action" {
  model_uuid       = juju_model.development.uuid
  application_name = juju_application.traefik.name
  action_name      = "show-proxied-endpoints"
}

# The action output can be used by other resources. For example, to
# fetch a nested value from a JSON string result:
locals {
  application_proxied_url = jsondecode(juju_action.action.output.proxied-endpoints)[juju_application.traefik.name].url
}
