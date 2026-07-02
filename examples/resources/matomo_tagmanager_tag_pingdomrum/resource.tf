
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example PingdomRUM Site"
  urls = ["https://example-pingdomrum.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example PingdomRUM Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example PingdomRUM Trigger"
}

resource "matomo_tagmanager_tag_pingdomrum" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-pingdomrum"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  pingdom_rom_id = "example-value"
}
