
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example UserInteraction Site"
  urls = ["https://example-userinteraction.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example UserInteraction Container"
}

resource "matomo_tagmanager_trigger_userinteraction" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-userinteraction"
}
