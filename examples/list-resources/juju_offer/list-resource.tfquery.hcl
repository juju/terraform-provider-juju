list "juju_offer" "this" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = "<model-uuid>"

    # Optional: filter by offer URL
    # offer_url = "admin/my-model.my-offer"
  }
}
