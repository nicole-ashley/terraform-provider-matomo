
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example GoogleAnalytics4 Site"
  urls = ["https://example-googleanalytics4.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example GoogleAnalytics4 Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example GoogleAnalytics4 Trigger"
}

resource "matomo_tagmanager_tag_googleanalytics4" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-googleanalytics4"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  measurement_id = "example-value"
}
