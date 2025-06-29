# Copyright (c) HashiCorp, Inc.

resource "appstoreconnect_user" "example" {
  first_name = "Oliver"
  last_name  = "Binns"

  email = "mail@oliverbinns.co.uk"

  all_apps_visible     = true
  provisioning_allowed = true
}