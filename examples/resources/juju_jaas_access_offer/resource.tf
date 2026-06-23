resource "juju_jaas_access_offer" "development" {
  offer_url        = juju_offer.myoffer.url
  access           = "consumer"
  users            = ["foo@domain.com"]
  groups           = [juju_jaas_group.development.uuid]
  idp_groups       = ["engineering"]
  service_accounts = ["Client-ID-1", "Client-ID-2"]
}
