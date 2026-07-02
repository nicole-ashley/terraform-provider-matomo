
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example WindowUnload Site"
  urls = ["https://example-windowunload.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example WindowUnload Container"
}

resource "matomo_tagmanager_trigger_windowunload" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-windowunload"
}
