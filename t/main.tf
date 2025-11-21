terraform {
  required_providers {
    juju = {
      source = "juju/juju"
    }
  }
}

provider "juju" {
   offering_controllers = {
     "offering" = {
       controller_addresses = "10.72.112.251:17070"
       username             = "admin"
       password             = "68c8d41f0f889dd56aa564331f81d3f5"
       ca_certificate       = "-----BEGIN CERTIFICATE-----\nMIIEEjCCAnqgAwIBAgIUCMpFSN50tGUDQBQLFhTC1ZGCmhowDQYJKoZIhvcNAQEL\nBQAwITENMAsGA1UEChMESnVqdTEQMA4GA1UEAxMHanVqdS1jYTAeFw0yNTExMjAx\nNzI3NThaFw0zNTExMjAxNzMyNThaMCExDTALBgNVBAoTBEp1anUxEDAOBgNVBAMT\nB2p1anUtY2EwggGiMA0GCSqGSIb3DQEBAQUAA4IBjwAwggGKAoIBgQDi2QJtmJM1\nEryJPD7lNr7bNBMwlPpIs0l/26yhppa4EzsDaY50jINKtMNSjHhXcTIaGignIvXN\na3bHD3N6XoXqM2M+p8+PEG+QxJ2OoOOn0+/PxaVkDJVzGPrG3Mq3RDAm7zgXYhIP\nAAzQ7QNHK+KtzY1OAqT1GlswuZTft7U5d+hurlEndQihrYRcBfGXPl2vZWHHTOJr\nXE5W1dTObstPUkdxho80hpJ/VABHtFOjJRjJ5v8KhB9exO3QZl25Q24RoiFHiZNn\nTH/TwFFhgF1ai4YA6CA4kWJVSCE9BAsYGOLumigSTyNAMt6Oxtabzd9awEkulaKe\nJVmK0fBRGOIv1QS6sBEsiTLYEpTUI6xbs+5u7tv1qTrO+iIG8XxB7a0yVhsq6mRH\nMgeISTb73fM+QcsiEAtqlheb0+4xuKUpzdua6FcgtgEZt6AkUrnyX+StxbUM75Fw\nKkVDS5CIE2AOH8YHzh4W12SvqkcIZ8kv0UusZA1yAq1NDqPXNnXasTECAwEAAaNC\nMEAwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFJlg\nxElMMSL9urXyfRNpST8bgieeMA0GCSqGSIb3DQEBCwUAA4IBgQDBSq4g2jnMqC6r\narEqCV3RNWe/K4Wir8UAUwu+u+VnyCKhSohHAEXujAS1nKghsWdU/7d6scD2+o1q\nFhPAbelNz/T8OTgSvc91BnoJns7VN45rMtQcVypjmwX8oqNsuxaZaO1Wofqx4ijb\ndRE0sxfi5OgaahNpmULaeea7RxcNFKhhNk9gcFgBa4gOwFSXEg9+jy37P0zJ/lkq\nrYAkX5prtK/r52dHH//iaKHhE6O6DZ21YO5BMaOpASxYHnrabI/1CfiCGoyz2YhV\nVCSnXey/YG8NAH4plsFqiZKC15jdsy6nERi1VSzUJwbq3jA1SwFrEnHqG/nhTHaf\nvpvrjDNns+hqKZtX20rJlyLKlwjp3dV6NE/7o7oT2BrVsHNujf+Ndz6OniyP8lvM\ntDsRejWGUg6HOUdPnXK5QrQb0yG2Rnt9Qvpht6t36Jjms+45xjndSvj1CObn0/GO\nWlqn+BkTV64acOfcS0dTU5ePEFcqeGyeE4FThBh91FqayHWnzIA=\n-----END CERTIFICATE-----\n"
     }
   }
}

data "juju_model" "machine_model" {
  name  = "admin"
  owner = "admin"
}

data "juju_application" "name" {
  model_uuid = data.juju_model.machine_model.uuid
  name       = "dummy-sink"
}

resource "juju_integration" "sink-source" {

  application {
    offering_controller = "offering"
    offer_url = "admin/offering.dummy-source"
    endpoint  = "sink"
  }

  application {
    name     = data.juju_application.name.name
    endpoint = "source"
  }

  model_uuid = data.juju_model.machine_model.uuid
}

import {
  to = juju_integration.sink-source
  id = "${data.juju_model.machine_model.uuid}:dummy-sink:source:dummy-source:sink"
}
