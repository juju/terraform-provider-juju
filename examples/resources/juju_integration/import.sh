# Integrations can be imported by using the format: model_name:provider_app_name:endpoint:requirer_app_name:endpoint.
# For integrations with an offer url, replace the requirer_app_name with the remote application name. The remote app
# name can be found by running `juju status` and looking for the SAAS heading.
# For example:
$ terraform import juju_integration.wordpress_db development:percona-cluster:server:wordpress:db
