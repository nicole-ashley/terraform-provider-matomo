
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example CustomEvent Site"
  urls = ["https://example-customevent.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example CustomEvent Container"
}

resource "matomo_tagmanager_trigger_customevent" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-customevent"
  event_name = "example-value"
}
