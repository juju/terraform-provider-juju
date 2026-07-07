data "juju_action" "action" {
  model_uuid = juju_model.development.uuid
  action_id  = juju_action.action.action_id
}

# The action output can be used by other resources. For example, to
# fetch a nested value from a JSON string result:
locals {
  application_proxied_url = jsondecode(data.juju_action.action.output.proxied-endpoints)[juju_application.traefik.name].url
}
