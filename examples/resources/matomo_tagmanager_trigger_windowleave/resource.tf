
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example WindowLeave Site"
  urls = ["https://example-windowleave.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example WindowLeave Container"
}

resource "matomo_tagmanager_trigger_windowleave" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-windowleave"
}
