## 0.6.0 (March 16, 2023)

NOTES:

* The Juju API is upgraded to 2.9.42

FEATURES:

* **New resource**: `juju_machine` enable users to provision machines using juju. (Thanks @jadonn)
* **New data source**: `juju_machine` enable users to incorporate already provisioned machines to their plans. (Thanks @gboutry)

ENHANCEMENTS:

* Applications now have a `placement` directive to indicate target machines.

BUG FIXES:

* Fixed parsing problem with ED25519 ssh keys. (Thanks @jsimpso)
* Fixed wrong application import due to inconsistent order of elements in application placement

## 0.5.0 (February 23, 2023)

NOTES:

* The Juju API is upgraded to 2.9.38.
* At this moment the manipulation of users may lead to problematic situations as Juju only disables users instead of removing them. A new release will be done when [LP2007258](https://bugs.launchpad.net/juju/+bug/2007258) is addressed. Meanwhile, proceed with caution.
* Once an SSH key has been added to a model, Juju does not allow all the SSH keys to be removed. In order to bypass this limitation, the provider does not remove an SSH key if it is the last one and displays a warning message informing about it.

FEATURES:

* **New resource**: `juju_user`
* **New resource**: `juju_credential`
* **New resource**: `juju_ssh_key`
* Cross-model relations can be set using the `via` argument.

## 0.4.2 (November 17, 2022)

NOTES:

* Added support for Juju 2.9.37
* Fix issue [96](https://github.com/juju/terraform-provider-juju/issues/96)
* Fix issue [95](https://github.com/juju/terraform-provider-juju/issues/95)

## 0.4.1 (August 25, 2022)

NOTES:

* The provider now receives the values for the `CharmConfig` while reading an already deployed application's status.

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
