
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example AllElementsClick Site"
  urls = ["https://example-allelementsclick.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example AllElementsClick Container"
}

resource "matomo_tagmanager_trigger_allelementsclick" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-allelementsclick"
}
