
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Drift Site"
  urls = ["https://example-drift.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Drift Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example Drift Trigger"
}

resource "matomo_tagmanager_tag_drift" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-drift"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  drift_id = "example-value"
}
