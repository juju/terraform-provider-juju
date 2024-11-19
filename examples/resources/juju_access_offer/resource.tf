resource "juju_access_offer" "this" {
  offer_url = juju_offer.my_application_offer.url
  consume   = [juju_user.dev.name]
}
