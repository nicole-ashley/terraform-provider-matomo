
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example GoogleAnalytics4Event Site"
  urls = ["https://example-googleanalytics4event.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example GoogleAnalytics4Event Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example GoogleAnalytics4Event Trigger"
}

resource "matomo_tagmanager_tag_googleanalytics4event" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-googleanalytics4event"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  event_name = "example-value"
}
