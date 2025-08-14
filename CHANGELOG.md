## 0.21.1 (August 4, 2025)

NOTES:

* **This release requires Juju controller version 2.9.49 or higher Juju.**
* **If using JAAS, this release requires Juju controller version 3.6.5 or higher.**
* This release uses Juju client api code from the Juju 3.6.4 release.
* This is a patch release to provide an early fix for [810](https://github.com/juju/terraform-provider-juju/issues/810).

BUG FIXES

* Avoid revoking and re-granting users' access to offers [816](https://github.com/juju/terraform-provider-juju/pull/816) by @SimoneDutto
* Allow offers that were previously consumed with the Juju CLI to work with Terraform [802](https://github.com/juju/terraform-provider-juju/pull/802) by @kian99

DOCUMENTATION

* Changes to move the docs to the Ubuntu domain [807](https://github.com/juju/terraform-provider-juju/pull/807) by @tmihoc
* Add sitemap config and support for google analytics [812](https://github.com/juju/terraform-provider-juju/pull/812) by @tmihoc

## 0.21.0 (July 21, 2025)

NOTES:

* **This release requires Juju controller version 2.9.49 or higher Juju.**
* **If using JAAS, this release requires Juju controller version 3.6.5 or higher.**
* This release uses Juju client api code from the Juju 3.6.4 release.

ENHANCEMENTS

* Retry model creation when error is `TransactionAborted` [763](https://github.com/juju/terraform-provider-juju/pull/763) by @SimoneDutto
* Wait for `hostname` field to be populated when creating `juju_machine` resource [788](https://github.com/juju/terraform-provider-juju/pull/788) by @SimoneDutto
* Add a `no-service-account` flag to the `juju_kubernetes_cloud` resource to avoid service account creation by the provier [793](https://github.com/juju/terraform-provider-juju/pull/793) by @SimoneDutto
* Using the custom type for `constraints` in the `juju_application` resource [796](https://github.com/juju/terraform-provider-juju/pull/796) by @alesstimec
* Waiting for machine deletion [761](https://github.com/juju/terraform-provider-juju/pull/761) by @alesstimec

BUG FIXES

* Store `juju_machine` ID immediately after creation [799](https://github.com/juju/terraform-provider-juju/pull/799) by @kian99

DOCUMENTATION

* Update `juju_offer` documentation to use `endpoints` [781](https://github.com/juju/terraform-provider-juju/pull/781) by @SimoneDutto

CI IMPROVEMENTS

* Fix UpgradeProvider tests by removing `PlanOnly = true` [782](https://github.com/juju/terraform-provider-juju/pull/782) by @SimoneDutto
* Fix JAAS integration test [792](https://github.com/juju/terraform-provider-juju/pull/792) by @SimoneDutto
* Avoid model creation in JAAS tests for `juju_kubernetes_cloud` [797](https://github.com/juju/terraform-provider-juju/pull/797) by @SimoneDutto

## 0.20.0 (June 16, 2025)

NOTES:

* **This release requires Juju controller version 2.9.49 or higher Juju.**
* **If using JAAS, this release requires Juju controller version 3.6.5 or higher.**
* This release uses Juju client api code from the Juju 3.6.4 release.

ENHANCEMENTS

* **BREAKING CHANGE** to the `juju_offer` schema. The `endpoint` field is field has been removed in favor of the `endpoints` field, which allows for definition of multiple endpoints in a single juju application offer [752](https://github.com/juju/terraform-provider-juju/pull/752) by @SimoneDutto.
* Introduction of `juju_machine` annotations [748](https://github.com/juju/terraform-provider-juju/pull/748) by @alesstimec.
* Introduction of waiting for changes to take effect [738](https://github.com/juju/terraform-provider-juju/pull/738) by SimoneDutto, followed up by [738](https://github.com/juju/terraform-provider-juju/pull/742) that introduces a wait for application deletion and [760](https://github.com/juju/terraform-provider-juju/pull/760) that introduces a wait for integration deletion.
* Removal of the `juju_jaas_access_service_account` resource [759](https://github.com/juju/terraform-provider-juju/pull/759) by @kian99. Service account authentication can now be used with JAAS to upload cloud credentials directly to the controller. 


BUG FIXES

* Fix for scaling up applications [730](https://github.com/juju/terraform-provider-juju/pull/730) by @alesstimec.
* Introduction of a custom type for `juju_machine` constraints [739](https://github.com/juju/terraform-provider-juju/pull/739) by @SimoneDutto, which fixes issues [#734](https://github.com/juju/terraform-provider-juju/issues/734) and [#729](https://github.com/juju/terraform-provider-juju/issues/729).

DOCUMENTATION

* Introduction of auto-generated RTD documentation [758](https://github.com/juju/terraform-provider-juju/pull/758) by @SimoneDutto.
* JAAS-related documentation improvements [746](https://github.com/juju/terraform-provider-juju/pull/746) by @tmihoc.

CI IMPROVEMENTS

* Update to the acceptance test to upload cloud credentials to the controller at the start of test [737](https://github.com/juju/terraform-provider-juju/pull/737) by @kian99.
* Allowing tests to diable waiting for changes to take effect [751](https://github.com/juju/terraform-provider-juju/pull/751) by @kian99.


## 0.19.0 (April 22, 2025)

NOTES:

* **This release requires Juju controller version 2.9.49 or higher Juju.**
* **If using JAAS, this release requires Juju controller version 3.5.0 or higher.**
* This release uses Juju client api code from the Juju 3.6.4 release.

ENHANCEMENTS

* Support for resetting application configuration settings [694](https://github.com/juju/terraform-provider-juju/pull/694) by @Soundarya03
* Machine creation timeout increased to 30 minutes [717](https://github.com/juju/terraform-provider-juju/pull/717) by @alesstimec
* Introduction of `machines` in `juju_application` to replace the now deprecated `placement` [716](https://github.com/juju/terraform-provider-juju/pull/716) by @alesstimec

BUG FIXES

* Fix for custom OCI images in application [700](https://github.com/juju/terraform-provider-juju/pull/700) by @SimoneDutto
* Fix for application resource update also updating charm revision number [709](https://github.com/juju/terraform-provider-juju/pull/709) by @SimoneDutto 


DOCUMENTATION

* Contribution guide [719](https://github.com/juju/terraform-provider-juju/pull/719) by @tmihoc
* Updates to the documentation home page and the community section [718](https://github.com/juju/terraform-provider-juju/pull/718) by @tmihoc

CI IMPROVEMENTS

* Update JAAS dependency to latest [693](https://github.com/juju/terraform-provider-juju/pull/693) by @SimoneDutto
* Update Juju dependency to 3.6.4 [696](https://github.com/juju/terraform-provider-juju/pull/696) by @kian99
* Update the CLA workflow to v2 [702](https://github.com/juju/terraform-provider-juju/pull/702) by @SimoneDutto
* Use the 3/stable channel for Juju in jaas integration tests [720](https://github.com/juju/terraform-provider-juju/pull/720) by @SimoneDutto

## 0.18.0 (March 18, 2025)

NOTES:

* **This release requires Juju controller version 2.9.49 or higher juju.**
* **If using JAAS, this release requires Juju controller version 3.5.0 or higher.**
* This release uses juju client api code from the juju 3.6.3 release.

ENHANCEMENTS:

* Support for adding annotations to the `juju_model` resource [689](https://github.com/juju/terraform-provider-juju/pull/689) by @SimoneDutto
* Support for JAAS roles [648](https://github.com/juju/terraform-provider-juju/pull/648) by @SimoneDutto
* Creating `juju_machine` resource blocks until machine reaches `running` state, which means `terraform apply` might now take longer to complete, but will enable correct and repeatable deploys in the future [679](https://github.com/juju/terraform-provider-juju/pull/679) by @alesstimec

DOCUMENTATION:

* Added how-tos for all resources and general documentation updates [658](https://github.com/juju/terraform-provider-juju/pull/658) by @tmihoc
* Updated role management howto [687](https://github.com/juju/terraform-provider-juju/pull/687) by @tmihoc

## 0.17.0 (February 18, 2025)

NOTES:

* **This release requires Juju controller version 2.9.49 or higher juju.**
* **If using JAAS, this release requires Juju controller version 3.5.0 or higher.**
* This release uses juju client api code from the juju 3.6.0 release.


BUG FIXES:

* fix for the update of the juju_kubernetes_cloud resource [#665](https://github.com/juju/terraform-provider-juju/pull/665) which addresses issue [#664](https://github.com/juju/terraform-provider-juju/issues/664).
* fix for the update of the juju_application resource to a specific charm revision [#669](https://github.com/juju/terraform-provider-juju/pull/669).
* fix for changing the juju_application charm base [#652](https://github.com/juju/terraform-provider-juju/pull/652), which addresses issue [#635](https://github.com/juju/terraform-provider-juju/issues/635)

## 0.16.0 (January 20, 2025)

NOTES:

* **This release requires Juju controller version 2.9.49 or higher juju.**
* **If using JAAS, this release requires Juju controller version 3.5.0 or higher.**
* This release uses juju client api code from the juju 3.6.0 release.

ENHANCEMENTS:

* The new `juju_application` data source allows you to incorporate already existing applications. (Thanks @shipperizer).
* The new `juju_access_offer` resource allows you to manage access to application offers (Tanks @amandahla).
* The new `juju_jaas_group` data source allows you to incorporate already existing groups (Thanks @pkulik0).

BUG FIXES:

* fix: fix the use of a hardcoded admin user #653

## 0.15.1 (December 3, 2024)

NOTES:

ENHANCEMENTS:
* Various documentation updates
  * add text to attributes where changing them will replace the resource
  * add warning requested in an issue Deleting config from plan does not reset it #393
  * reword note on offer_url in integration resources

## 0.15.0 (October 14, 2024)

NOTES:

* **This release requires Juju controller version 2.9.49 or higher juju.**
* **If using JAAS, this release requires Juju controller version 3.5.0 or higher.**
* This release uses juju client api code from the juju 3.5.1 release.

ENHANCEMENTS:

* Support for JAAS access settings via the following new resources:
    *  `juju_jaas_access_cloud`
    *  `juju_jaas_access_controller`
    *  `juju_jaas_access_group`
    *  `juju_jaas_access_model`
    *  `juju_jaas_access_offer`
    *  `juju_jaas_access_service_account`
    *  `juju_jaas_group`
* Support for adding kubernetes clouds to existing controllers with the new
`juju_kubernetes_cloud` resource.

BUG FIXES:

* feat: add computed uuid to model resource by @hmlanigan in #599
* fix: remove requirement on ca cert by @kian99 in #567

## 0.14.0 (September 9, 2024)

NOTES:

* **This release requires juju controller version 2.9.49 or later juju.**
* This release uses juju client api code from the juju 3.5.1 release.

ENHANCEMENTS:

* Allow applications to specify charm resources as oci-image urls, in addition to revisions.

BUG FIXES:

* fix: bugs 535 and 539 only find storage for application being read by @hmlanigan in #557
* fix (application): do no panic on nil pointer by @hmlanigan in #563
* fix(constraints): ensure constraints are non-null by @jack-w-shaw in #556
* fix(storagedirective): fix storage directive validator by @anvial in #538
* fix(application): update resource example to use correct charm storage by @anvial in #533

## 0.13.0 (July 15, 2024)

NOTES:

* **This release requires juju controller version 2.9.49 or later juju.**
* This release uses juju client api code from the juju 3.5.1 release.

ENHANCEMENTS:

* Support for application resources to use storage is added. You can now use `storage` and `storage_directives` fields in an application resource to utilize storage in your application.
* Bug reports are enhanced to require more information, including the Juju controller version.
* [Conventional commits](https://www.conventionalcommits.org/en/v1.0.0/) are now required in contributions to the repository.
* Provider test runs are improved to run faster. A foundation for running tests in parallel has been laid out. More work is needed on GitHub runners to enable parallel testing.

BUG FIXES:

* Panic in handling storage conversion errors is fixed, by @anvial in https://github.com/juju/terraform-provider-juju/pull/525
* Placement and constraints directives are allowed to co-exist in machine resources. Added by @hmlanigan in https://github.com/juju/terraform-provider-juju/pull/499

## 0.12.0 (April 22, 2024)

NOTES:

* **This release requires juju controller version 2.9.49 or later juju.**
* This release uses juju client api code from the juju 3.5-beta1 candidate release.
* The added JAAS login enhancements requires Juju controller version 3.5.0 or higher.
* The added Juju secrets support requires Juju controller version 3.4.0 or higher.

ENHANCEMENTS:

* Support for user secret management is added. You can now use `juju_secret` and `juju_access_secret` resources to create and manage secrets, as well as grant/revoke access to the applications in your plan. You can also use `juju_secret` data source in your configuration to access a secret.
* Provider config is enhanced with support for Client ID and secret to enable logging in to JAAS using client credentials.

BUG FIXES:

* Channel for charms in an application resource requires both track and risk (e.g., `latest` vs `latest/stable`). A validation for channel in application resource is added by @Aflynn50 in https://github.com/juju/terraform-provider-juju/pull/447


## 0.11.0 (March 18, 2024)

NOTES:

* **This release requires juju controller version 2.9.47 or later juju.**
* This release uses juju client api code from the juju 3.3.0 release. 

ENHANCEMENTS:

* Add resource revisions for juju_application. This is similar to 
`juju deploy <charm> --resource <name>:#` and `juju attach-resource <application> <name>:#`.
* Add kvm and/or lxd machines via the juju_machine resource. This is similar to 
`juju add-machine lxd` and `juju add-machine kvm:0` commands.
* Use the DeployFromRevision API endpoint from juju for application deployments with juju 3.3+.
* Add space support for the juju_application resources. You can now specify endpoint bindings for
applications. This is similar to `juju deploy --bind` and `juju bind` commands.

BUG FIXES:

* Fix upgrade charm revision for application resources by @hmlanigan in https://github.com/juju/terraform-provider-juju/pull/414
* Fixes Config/Revision update ordering. by @anvial in https://github.com/juju/terraform-provider-juju/pull/407
* Adds error check to ReadModel function by @anvial in https://github.com/juju/terraform-provider-juju/pull/416
* Add info about `plangenerator` to README. by @anvial in https://github.com/juju/terraform-provider-juju/pull/429
* Update retry to improve machine placement on read and introduce internal/juju unit tests with mocking by @hmlanigan in https://github.com/juju/terraform-provider-juju/pull/433

## 0.10.1 (January 12, 2024)

BUG FIXES:

* Do not require permissions on the controller model to deploy an application by @hmlanigan in https://github.com/juju/terraform-provider-juju/pull/353
* Handle resolved charm origins without base by @hmlanigan in https://github.com/juju/terraform-provider-juju/pull/375
* Find operating system for deploy regardless of juju controller version by @hmlanigan in https://github.com/juju/terraform-provider-juju/pull/358
* Populating Juju controller config no longer immediately fails if Juju cli does not exist by @Osama-Kassem

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
