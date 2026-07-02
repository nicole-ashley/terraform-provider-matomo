
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example TawkTo Site"
  urls = ["https://example-tawkto.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example TawkTo Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example TawkTo Trigger"
}

resource "matomo_tagmanager_tag_tawkto" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-tawkto"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  tawk_to_id = "example-value"
  tawk_to_widget_id = "example-value"
}
