# Integrations can be imported by using the format: model_name:provider_app_name:endpoint:requirer_app_name:endpoint, for example:
$ terraform import juju_integration.wordpress_db development:percona-cluster:server:wordpress:db

# For integration with an offer url, import by using the remote application name. The remote app name depends on the offer url
# and whether an alias was provided. E.g. an offer URL of `admin/dbModel.mysql` will create a remote app called `mysql` 
# but you may provide an alias.
$ terraform import juju_integration.wordpress_db development:percona-cluster:mysql:remote
