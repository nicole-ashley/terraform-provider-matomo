
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Shareaholic Site"
  urls = ["https://example-shareaholic.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Shareaholic Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example Shareaholic Trigger"
}

resource "matomo_tagmanager_tag_shareaholic" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-shareaholic"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  shareaholic_site_id = "example-value"
}
