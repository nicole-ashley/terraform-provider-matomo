
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Honeybadger Site"
  urls = ["https://example-honeybadger.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Honeybadger Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example Honeybadger Trigger"
}

resource "matomo_tagmanager_tag_honeybadger" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-honeybadger"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  honeybadger_api_key = "example-value"
}
