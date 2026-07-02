
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Axeptio Site"
  urls = ["https://example-axeptio.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Axeptio Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example Axeptio Trigger"
}

resource "matomo_tagmanager_tag_axeptio" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-axeptio"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  project_id = "example-value"
}
