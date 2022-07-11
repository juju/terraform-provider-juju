## 0.3.0 (unreleased)

## 0.2.0 (July 11, 2022)

NOTES:

* The proposed ressource `juju_deployment` has been replaced with `juju_application` as it was deemed that this fit better with the vocabulary used by JuJu

FEATURES:

* **New Resource:** `juju_application`

## 0.1.0 (June 27, 2022)

NOTES:

* provider: The provider has a dependency on Juju CLI configuration store. It expects configuration to be found in either `$XDG_DATA_HOME/juju` or `~/.local/share/juju`.

FEATURES:

* **New Data Source:** `juju_model`
* **New Resource:** `juju_model`
