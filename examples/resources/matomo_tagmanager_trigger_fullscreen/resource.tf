
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Fullscreen Site"
  urls = ["https://example-fullscreen.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Fullscreen Container"
}

resource "matomo_tagmanager_trigger_fullscreen" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-fullscreen"
  trigger_action = "any"
}
