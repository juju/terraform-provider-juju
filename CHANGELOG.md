## 0.4.0 (August 19, 2022)

FEATURES:

* **Application expose** is now available.

NOTES:

* Now the provider considers the current status of `expose` and `config` for any application. This means, that when a plan is applied, this is compared with the current status returned by Juju and applied when required. For example, an exposed application can be manually unexposed using the Juju CLI. When applying the plan again, the provider will detect a mismatch and expose the application again.
* Following the previous note, the application configuration returned by Juju can contain more elements than those the plan is aware of. For this reason, the provider will only consider those parameteres already specified in the plan. This means that if any configuration parameter is manually set using the Juju CLI and this parameter is not set in the plan, no changes will be applied by the provider.

## 0.3.1 (July 18, 2022)

NOTES:

* provider: The provider has a dependency on Juju CLI configuration store. It expects configuration to be found in either `$XDG_DATA_HOME/juju` or `~/.local/share/juju`.

BUG FIXES

* resource/juju_application: Avoid inconsistency with a Charm's self-reported name
* resource/juju_application: Fix error encountered when changing units whilst operating on a CAAS model

## 0.3.0 (July 14, 2022)

NOTES:

* provider: The provider has a dependency on Juju CLI configuration store. It expects configuration to be found in either `$XDG_DATA_HOME/juju` or `~/.local/share/juju`.

FEATURES:

* **New Resource** `juju_integration`

BUG FIXES

* resource/juju_application: If a malformed id is supplied during import then return an error message instead of panicking.

## 0.2.0 (July 11, 2022)

NOTES:

* provider: The provider has a dependency on Juju CLI configuration store. It expects configuration to be found in either `$XDG_DATA_HOME/juju` or `~/.local/share/juju`.

FEATURES:

* **New Resource** `juju_application`

IMPROVEMENTS

* resource/juju_model: Ensure that when entries are removed from `config` that they are unset in the model configuration


## 0.1.0 (June 27, 2022)

NOTES:

* provider: The provider has a dependency on Juju CLI configuration store. It expects configuration to be found in either `$XDG_DATA_HOME/juju` or `~/.local/share/juju`.

FEATURES:

* **New Data Source:** `juju_model`
* **New Resource:** `juju_model`
