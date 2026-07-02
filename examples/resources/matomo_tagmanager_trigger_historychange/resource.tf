
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example HistoryChange Site"
  urls = ["https://example-historychange.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example HistoryChange Container"
}

resource "matomo_tagmanager_trigger_historychange" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-historychange"
}
