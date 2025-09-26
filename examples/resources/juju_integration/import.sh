# Integrations can be imported by using the format: model_uuid:provider_app_name:endpoint:requirer_app_name:endpoint.
# For integrations with an offer url, replace the requirer_app_name with the remote application name. The remote app
# name can be found by running `juju status` and looking for the SAAS heading.
# For example:
$ terraform import juju_integration.wordpress_db 4b6bd192-13bb-489d-b7a7-06f6efc2928d:percona-cluster:server:wordpress:db
