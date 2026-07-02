
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Matomo Site"
  urls = ["https://example-matomo.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Matomo Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example Matomo Trigger"
}

resource "matomo_tagmanager_tag_matomo" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-matomo"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  matomo_config = "example-value"
  tracking_type = "event"
  event_category = "example-value"
  event_action = "example-value"
}
