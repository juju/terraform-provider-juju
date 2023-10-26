## 0.10.0 (October 26, 2023)

NOTES:

* **Use of Series for machine and application resources is now deprecated.** Use Base instead.
* Principal in the `juju_application` resource schema has been deprecated. It was computed only data, is not needed, and has no replacement.

FEATURES:

* **Add base support for machines and applications**: `juju_application` and `juju_machine` now support bases. 
A base is an alternative way to specify which operating system and version a charm or machine should be deployed 
with. For example `ubuntu@22.04`.
* Use of the 2.9.45 code base for juju API calls.
* Stable integration tests have been replaced with testing of upgrading from the last release to the current code.
* There is now a github action to use a loadbalancer as a tunnel to juju controller on k8s.

BUG FIXES:

* Gracefully fail to deploy a bundle @hmlanigan in https://github.com/juju/terraform-provider-juju/pull/318
* Fix remove (cmr) integration with multiple consumers by @hemanthnakkina in https://github.com/juju/terraform-provider-juju/issues/308
* Update charm resources if necessary when updating a charm by @cderici in https://github.com/juju/terraform-provider-juju/pull/326
* Application resource plans hitting RequiresReplace can fail with "application already exists AND

  Application resource plans can fail with "Charm Already Exists", if another application has loaded that charm to the controller before

  by @hmlanigan in https://github.com/juju/terraform-provider-juju/pull/329




## 0.9.1 (September 22, 2023)

BUG FIXES:

* Credential name doesn't work since 0.9.0 by @hmlanigan in https://github.com/juju/terraform-provider-juju/pull/312

## 0.9.0 (September 7, 2023)

ENHANCEMENTS:

* Migration from the Terraform SDK to the Terraform Framework for plugins

BUG FIXES:

* Provider panics when generating plan by @hmlanigan as part of https://github.com/juju/terraform-provider-juju/pull/265

## 0.8.0 (June 13, 2023)

FEATURES:

* **Add provisioned machines**: `juju_machine` now supports machines already provisioned. This is similar to using `juju add-machine ssh:user@host`. This new feature enables other machines already provisioned using Terraform to be added to a Juju controller.

ENHANCEMENTS:

* The CI has been enhanced by enabling a K8s based Juju controller for GitHub actions.
* Integration tests now can use environment variables to identify if the testing environment has everything required to run the test. For example, a K8s controller vs an LXD controller.

BUG FIXES:

* Process region value in cloud models and force it to be computed by @juanmanuel-tirado in https://github.com/juju/terraform-provider-juju/pull/214
* [JUJU-3905] Upgrade charms using channel by @juanmanuel-tirado in https://github.com/juju/terraform-provider-juju/pull/224
* Fix message for model not found (#222) by @amandahla in https://github.com/juju/terraform-provider-juju/pull/227
* Support models removed outside the plan. by @juanmanuel-tirado in https://github.com/juju/terraform-provider-juju/pull/229

## 0.7.0 (May 12, 2023)

Notes:

* This is a periodic release with some bug fixes.

FEATURES:

* **New data source**: `juju_offer` enable users to incorporate already existing offers. (Thanks @gboutry)

BUG FIXES:

* Wait for apps before integrate by @juanmanuel-tirado in https://github.com/juju/terraform-provider-juju/pull/189
* Remove integration from state if it was removed manually (#186) by @amandahla in https://github.com/juju/terraform-provider-juju/pull/192
* Add OwnerName to ApplicationOfferFilter by @hemanthnakkina in https://github.com/juju/terraform-provider-juju/pull/201
* Remove Application,Machine,Model and Offer from state if it was removed manually by @amandahla in https://github.com/juju/terraform-provider-juju/pull/205
* [JUJU-3654] Added ApplicationNotFound error for better error control. by @juanmanuel-tirado in https://github.com/juju/terraform-provider-juju/pull/206
* [JUJU-3315] Force "stable" channel to be "latest/stable" when reading apps. by @juanmanuel-tirado in https://github.com/juju/terraform-provider-juju/pull/204


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
