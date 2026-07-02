
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example VisualWebsiteOptimizer Site"
  urls = ["https://example-visualwebsiteoptimizer.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example VisualWebsiteOptimizer Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example VisualWebsiteOptimizer Trigger"
}

resource "matomo_tagmanager_tag_visualwebsiteoptimizer" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-visualwebsiteoptimizer"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  account_id = "example-value"
}
