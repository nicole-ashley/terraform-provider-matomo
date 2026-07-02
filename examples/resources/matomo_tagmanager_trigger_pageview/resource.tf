
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example PageView Site"
  urls = ["https://example-pageview.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example PageView Container"
}

resource "matomo_tagmanager_trigger_pageview" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-pageview"
}
