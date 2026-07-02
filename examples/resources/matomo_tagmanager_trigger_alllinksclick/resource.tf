
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example AllLinksClick Site"
  urls = ["https://example-alllinksclick.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example AllLinksClick Container"
}

resource "matomo_tagmanager_trigger_alllinksclick" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-alllinksclick"
}
