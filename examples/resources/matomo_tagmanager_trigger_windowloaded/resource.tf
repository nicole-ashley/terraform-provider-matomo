
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example WindowLoaded Site"
  urls = ["https://example-windowloaded.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example WindowLoaded Container"
}

resource "matomo_tagmanager_trigger_windowloaded" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-windowloaded"
}
