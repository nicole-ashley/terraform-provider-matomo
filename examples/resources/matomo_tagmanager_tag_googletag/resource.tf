
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example GoogleTag Site"
  urls = ["https://example-googletag.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example GoogleTag Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example GoogleTag Trigger"
}

resource "matomo_tagmanager_tag_googletag" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-googletag"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  google_tag_id = "example-value"
}
