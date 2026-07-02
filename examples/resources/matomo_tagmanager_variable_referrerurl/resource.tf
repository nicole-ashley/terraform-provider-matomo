
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example ReferrerUrl Site"
  urls = ["https://example-referrerurl.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example ReferrerUrl Container"
}

resource "matomo_tagmanager_variable_referrerurl" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-referrerurl"
  url_part = "host"
}
