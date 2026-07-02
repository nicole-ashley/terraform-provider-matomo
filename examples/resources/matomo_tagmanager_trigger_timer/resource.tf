
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Timer Site"
  urls = ["https://example-timer.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Timer Container"
}

resource "matomo_tagmanager_trigger_timer" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-timer"
  trigger_interval = 1
}
