
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example ScrollReach Site"
  urls = ["https://example-scrollreach.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example ScrollReach Container"
}

resource "matomo_tagmanager_trigger_scrollreach" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-scrollreach"
  scroll_type = "horizontalpercentage"
}
