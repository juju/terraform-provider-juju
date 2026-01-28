## 1.2.0

NOTES:

* **This release requires Juju controller version 2.9.49 or higher Juju.**
* **If using JAAS, this release requires Juju controller version 3.6.5 or higher.**
* This release uses Juju client api code from the Juju 3.6.11 release.

ENHANCEMENTS

* Addition of the `juju_cloud` resource by @ale8k in [#1012](https://github.com/juju/terraform-provider-juju/pull/1012)
* Implementation of the `juju_cloud` CRUD methods by @ale8k in [#1009](https://github.com/juju/terraform-provider-juju/pull/1009)
* Addition of the `target_controller` field to the `juju_model` resource by @kian99 in [#1011](https://github.com/juju/terraform-provider-juju/pull/1011)

BUG FIXES

* A fix for unsetting removed/deprecated configuration values from `juju_application` by @SimoneDutto in [#992](https://github.com/juju/terraform-provider-juju/pull/992)
* A fix for referencing model data source in locals by @kian99 in [#1020]https://github.com/juju/terraform-provider-juju/pull/1020)
* Removal of custom Charmhub client for legacy deploys by @alesstimec in [#987](https://github.com/juju/terraform-provider-juju/pull/987)

DOCUMENTATION

* Documentation of differences in `juju_application`'s  `unit` values in 0.19 and later versions by @ale8k in [#1004](https://github.com/juju/terraform-provider-juju/pull/1004)
* Upgrade to the Sphinx starter pack by @tmihoc in [#1010](https://github.com/juju/terraform-provider-juju/pull/1010)

CI & MAINTENANCE

* Fix for the Juju 2.9 integration test by @luci1900 in [#1014](https://github.com/juju/terraform-provider-juju/pull/1014)


## 1.1.1

NOTES:

* **This release requires Juju controller version 2.9.49 or higher Juju.**
* **If using JAAS, this release requires Juju controller version 3.6.5 or higher.**
* This release uses Juju client api code from the Juju 3.6.11 release.

BUG FIXES

* Fix for a panic when updating JAAS access resources by @kian99 in [#1001](https://github.com/juju/terraform-provider-juju/pull/1001)

DOCUMENTATION

* A documentation fix for a model name reference in documentation by @alesstimec in [#998](https://github.com/juju/terraform-provider-juju/pull/998)

## 1.1.0

NOTES:

* **This release requires Juju controller version 2.9.49 or higher Juju.**
* **If using JAAS, this release requires Juju controller version 3.6.5 or higher.**
* This release uses Juju client api code from the Juju 3.6.11 release.

ENHANCEMENTS

* Introduction of cross-controller integrations by @SimoneDutto in [#965](https://github.com/juju/terraform-provider-juju/pull/965)
* Addition of the `offering_controller` field to the `juju_offer` data source by @SimoneDutto in [#979](https://github.com/juju/terraform-provider-juju/pull/979)
* Implementation of a validator for `offer_url` in the `juju_integration` resource to reject controller source by @kian99 in [#975](https://github.com/juju/terraform-provider-juju/pull/975)
* Addition of client credentials to offering controllers in the provider config by @alesstimec in [#982](https://github.com/juju/terraform-provider-juju/pull/982)

BUG FIXES

* Ensuring resources are refreshed when a charm is updates by @kian99 in [#981](https://github.com/juju/terraform-provider-juju/pull/981)
* A fix for the cross-controller integration import by @luci1900 in [#980](https://github.com/juju/terraform-provider-juju/pull/980)
* A fix for exposing and unexposing `juju_application` by @alesstimec in [#951](https://github.com/juju/terraform-provider-juju/pull/951)
* Removing the use or patterns in Juju status calls to get application status by @alesstimec in [#957](https://github.com/juju/terraform-provider-juju/pull/957)
* Correct validation of nested application attribute by @kian99 in [#989](https://github.com/juju/terraform-provider-juju/pull/989)

DOCUMENTATION 

* Documentation for cross controller integrations by @SimoneDutto in [#977](https://github.com/juju/terraform-provider-juju/pull/977)
* Documentation for cross controller integration import by @luci1900 in [#985](https://github.com/juju/terraform-provider-juju/pull/985)
* Update of documentation for v1.0.0 resources by @kian99 in [#964](https://github.com/juju/terraform-provider-juju/pull/964)
* Documentation about using the http provider with charmhub to compute charm revision from channel by @SimoneDutto in [#984](https://github.com/juju/terraform-provider-juju/pull/984)

  
CI & MAINTENANCE

* Improvement of the CI speed and resource usage by @kian99 in [#897](https://github.com/juju/terraform-provider-juju/pull/897)
* Update the Juju dependency to 3.6.11 by @ale8k in [#958](https://github.com/juju/terraform-provider-juju/pull/958)
* A fix for CI naming and jaas tests by @kian99 in [#988](https://github.com/juju/terraform-provider-juju/pull/988)

## 1.0.0

NOTES:

* **This release requires Juju controller version 2.9.49 or higher Juju.**
* **If using JAAS, this release requires Juju controller version 3.6.5 or higher.**
* This release uses Juju client api code from the Juju 3.6.4 release.

BREAKING CHANGES:

* The following resources have had the `model` field replaced with `model_uuid` [791](https://github.com/juju/terraform-provider-juju/issues/791). Users will need to update their plans and modules to refer to models by UUID. Further details are available at the linked issue.
  * Application data source
  * Offer data source
  * Secret data source
  * Machine data source
  * Application resource
  * Integration resource
  * Machine resource
  * Offer resource
  * SSH Key resource
  * Model Access resource
  * Secret Access resource
  * Secret resource
* The `model` data-source has a new `owner` field. The data-source requires the `owner` field
is set in addition to the `name`. Alternatively you can lookup a model by UUID. 
Note that the `uuid` field cannot be set alongside `name`/`owner`.
* Related to the above, all resources that import a model-scoped resource have had their import
syntax changed to require a model UUID instead of a model name. This includes most of the resources
in the above list with some exceptions (e.g. offer import requires the offer URL) and additionally
includes the model resource.
* The offer data source no longer contains the computed field `model`.
* The `placement` field has been removed from the application resource - use `machines` instead.
* The `principle` field has been removed from the application resource as it was unused.
* The `series` field has been removed from the application resource - use `base` instead.
* The `series` field has been removed from the machine resource - use `base` instead.

UPGRADING PLANS:

We realize any breaking changes are painful, but a move from v0.23.0 to v1.0.0 is one of the few opportunities we have to do them. To make this transition easier we have developed an upgrader advisory tool that will take an existing tf file as input and replace references to model names with model uuids. 
This tool is intended as an advisory tool, although we have extensively tested the tool, make sure to carefully review proposed changes before committing to them.

The tool is located [here](https://github.com/juju/terraform-provider-juju/tree/main/juju-tf-upgrader) and to use it, one simply runs:
```
go run github.com/juju/terraform-provider-juju/juju-tf-upgrader path/to/file.tf
```

As always, please read the [README.md](https://github.com/juju/terraform-provider-juju/blob/main/juju-tf-upgrader/README.md) first and in case of any issues contact the team in our public [Matrix channel](https://matrix.to/#/#terraform-provider-juju:ubuntu.com). 

ENHANCEMENTS

* Allow deploying OCI charm resources from private registries by @kian99 in [#924](https://github.com/juju/terraform-provider-juju/pull/924).
* Handle partial app deployments by @kian99 in [#926](https://github.com/juju/terraform-provider-juju/pull/926).
* Add a storage pool resource by @ale8k in [#908](https://github.com/juju/terraform-provider-juju/pull/908).
* Add a storage pool data source by @ale8k in [#928]( https://github.com/juju/terraform-provider-juju/pull/928)

BUG FIXES

* Fix for [#671](https://github.com/juju/terraform-provider-juju/issues/671) by @ale8k in [#925](https://github.com/juju/terraform-provider-juju/pull/925)
* Making `storage` field in `juju_application` read only by @ale8k in [#943](https://github.com/juju/terraform-provider-juju/pull/943)
* Fix for resource update in `juju_application` by @kian99 in [#947](https://github.com/juju/terraform-provider-juju/pull/947)
    
DOCUMENTATION

* Documentation on plan upgrade to provider v1.0.0 by @kian99 in [#883]( https://github.com/juju/terraform-provider-juju/pull/883)
* Documentation on creating dependency in deployment by @yanksyoon in [#927](https://github.com/juju/terraform-provider-juju/pull/927)
* Update to `juju_application` resource examples by @kian99 in [#939](https://github.com/juju/terraform-provider-juju/pull/939)
* How to use Juju CLI in Terraform plans by @kian99 in [#949](https://github.com/juju/terraform-provider-juju/pull/949)
* Examples for `juju_ssh_key` from Github and Launchpad by @tmihoc in [#953](https://github.com/juju/terraform-provider-juju/pull/953)

CI & MAINTENANCE

* SBOM generation in CI by @alesstimec in [#919](https://github.com/juju/terraform-provider-juju/pull/919)

## 0.23.1 (October 9, 2025)

NOTES:

* **This release requires Juju controller version 2.9.49 or higher Juju.**
* **If using JAAS, this release requires Juju controller version 3.6.5 or higher.**
* This release uses Juju client api code from the Juju 3.6.4 release.

BUG FIXES

* Partial revert of new behaviour related to cross-model relations by @kian99 in [#938](https://github.com/juju/terraform-provider-juju/pull/938) that exposed a Juju bug, see [#20818](https://github.com/juju/juju/issues/20818).
  * Multiple saas apps for the same offer URL surface an issue where relations to the additional saas apps don't work correctly. See the Juju issue for more details on the problem and how to resolve it.

See our docs for more information https://documentation.ubuntu.com/terraform-provider-juju/latest/reference/terraform-provider/resources/integration/#cross-model-relations on the provider's approach to cross-model relations.

## 0.23.0 (September 22, 2025)

NOTES:

* **This release requires Juju controller version 2.9.49 or higher Juju.**
* **If using JAAS, this release requires Juju controller version 3.6.5 or higher.**
* This release uses Juju client api code from the Juju 3.6.4 release.

ENHANCEMENTS

* Added the `secret_uri` computed field to the secret resource by @alesstimec in [#850](https://github.com/juju/terraform-provider-juju/pull/850)
* Waiting for model resource to be deleted before returning by @SimoneDutto in [#743](https://github.com/juju/terraform-provider-juju/pull/743)
* Support `null` values in model config by @SimoneDutto in [#851](https://github.com/juju/terraform-provider-juju/pull/851)
* Support `null` values app config by @SimoneDutto in [#864](https://github.com/juju/terraform-provider-juju/pull/864)
* Added custom create timeout for the machine resource by @kian99 in [#868](https://github.com/juju/terraform-provider-juju/pull/868)
* Issue errors instead of warnings by default on failed resource deletion by @kian99 in [#877](https://github.com/juju/terraform-provider-juju/pull/877) - See the new provider config `skip_failed_deletion` to revert to the previous behavior - more information is available in the provider documentation.
* Allow changing charm channel and revision together by @luci1900 in [#889](https://github.com/juju/terraform-provider-juju/pull/889)

BUG FIXES

* Change for the `applications` field from list to set in the `juju_access_secret` resource by @alesstimec in [#848](https://github.com/juju/terraform-provider-juju/pull/848)
* Change for the `users` field from list to set in the `juju_access_model` resource by @alesstimec in [#849](https://github.com/juju/terraform-provider-juju/pull/849)
* Fix for [#267](https://github.com/juju/terraform-provider-juju/issues/267) affecting the `ssh-key` resource by @SimoneDutto in [#844](https://github.com/juju/terraform-provider-juju/pull/844)
* Fix for [#662](https://github.com/juju/terraform-provider-juju/issues/662) by @alesstimec in [#831](https://github.com/juju/terraform-provider-juju/pull/831)
* Clarification of the error when reading application offers by @claudiubelu in [#872](https://github.com/juju/terraform-provider-juju/pull/872)
* Fix for [#881](https://github.com/juju/terraform-provider-juju/issues/881) by @SimoneDutto in [#890](https://github.com/juju/terraform-provider-juju/pull/890)
* Fix for the offer and integration resource logic by @kian99 in [#893](https://github.com/juju/terraform-provider-juju/pull/893)
* Fix for finding offers based on endpoints by @SimoneDutto in [#906](https://github.com/juju/terraform-provider-juju/pull/906)
* Fix for [#473](https://github.com/juju/terraform-provider-juju/issues/473) and [#235](https://github.com/juju/terraform-provider-juju/issues/235) by @kian99 in [#898](https://github.com/juju/terraform-provider-juju/pull/898)

DOCUMENTATION

* Add channel and revision example and clarification to the charms by @tmihoc in [#892](https://github.com/juju/terraform-provider-juju/pull/892)
* Add doc on managing model migrations by @kian99 in [#895](https://github.com/juju/terraform-provider-juju/pull/895)

CI & MAINTENANCE

* Added wait-for and add unit tests in CI by @SimoneDutto in [#852](https://github.com/juju/terraform-provider-juju/pull/852)
* Remove microk8s setup for jaas test and refactor tests by @SimoneDutto in [#861](https://github.com/juju/terraform-provider-juju/pull/861)
* Re-enable the machine with placement test by @alesstimec in [#840](https://github.com/juju/terraform-provider-juju/pull/840)
* Added a script to generate env file from switched controller by @SimoneDutto in [#879](https://github.com/juju/terraform-provider-juju/pull/879)
* Use concurrency in workflows by @kian99 in [#894](https://github.com/juju/terraform-provider-juju/pull/894)
* Added the security scan workflow by @alesstimec in [#902](https://github.com/juju/terraform-provider-juju/pull/902)
* Added `SECURITY.md` by @alesstimec in [#904](https://github.com/juju/terraform-provider-juju/pull/904)
* Added tiobe scan workflow to point to the repo where we run it by @SimoneDutto in [#907](https://github.com/juju/terraform-provider-juju/pull/907)

## 0.22.0 (August 18, 2025)

NOTES:

* **This release requires Juju controller version 2.9.49 or higher Juju.**
* **If using JAAS, this release requires Juju controller version 3.6.5 or higher.**
* This release uses Juju client api code from the Juju 3.6.4 release.

ENHANCEMENTS

* An improvement of sematic comparison for constraints by @kian99 in [829](https://github.com/juju/terraform-provider-juju/pull/829).

BUG FIXES

* A fix for SSH key resource ID handling by @kian99 in [824](https://github.com/juju/terraform-provider-juju/pull/824).
* A fix for removal of multiple integrations with the same endpoint by @SimoneDutto in [814](https://github.com/juju/terraform-provider-juju/pull/814).

DOCUMENTATION

* Addition of related links by @tmihoc in [825](https://github.com/juju/terraform-provider-juju/pull/825).
* Clarification of cloud and controller authorization and improvement to documentation navigation by @tmihoc in [831](https://github.com/juju/terraform-provider-juju/pull/832).
* Update to the documentation starter pack by @tmihoc in [836](https://github.com/juju/terraform-provider-juju/pull/836).

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
