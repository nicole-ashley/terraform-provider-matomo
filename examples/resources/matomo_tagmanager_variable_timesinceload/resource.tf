
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example TimeSinceLoad Site"
  urls = ["https://example-timesinceload.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example TimeSinceLoad Container"
}

resource "matomo_tagmanager_variable_timesinceload" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-timesinceload"
  unit = "m"
}
