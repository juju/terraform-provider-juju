# A plan where the model resource is absent. model_uuid literals should be
# left in place and warnings emitted.

resource "juju_application" "all_apps_0" {
  provider   = juju
  model_uuid  = "c1cecf1e-fe66-4589-8585-e579edd6f58b"
  name        = "dummy-sink"
  machines    = ["1"]
  trust       = false
  charm {
    name = "juju-qa-dummy-sink"
  }
}

import {
  to       = juju_application.all_apps_0
  provider = juju
  identity = {
    id = "c1cecf1e-fe66-4589-8585-e579edd6f58b:dummy-sink"
  }
}

resource "juju_machine" "all_machines_0" {
  provider   = juju
  model_uuid = "c1cecf1e-fe66-4589-8585-e579edd6f58b"
  name       = "machine-1"
  machine_id = "1"
}

import {
  to       = juju_machine.all_machines_0
  provider = juju
  identity = {
    id = "c1cecf1e-fe66-4589-8585-e579edd6f58b:1:machine-1"
  }
}
