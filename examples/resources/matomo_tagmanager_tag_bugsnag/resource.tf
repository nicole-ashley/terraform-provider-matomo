
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Bugsnag Site"
  urls = ["https://example-bugsnag.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Bugsnag Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example Bugsnag Trigger"
}

resource "matomo_tagmanager_tag_bugsnag" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-bugsnag"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  api_key = "example-value"
}
