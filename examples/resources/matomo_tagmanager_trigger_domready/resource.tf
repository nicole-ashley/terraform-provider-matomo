
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example DomReady Site"
  urls = ["https://example-domready.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example DomReady Container"
}

resource "matomo_tagmanager_trigger_domready" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-domready"
}
