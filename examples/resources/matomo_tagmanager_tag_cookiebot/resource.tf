
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Cookiebot Site"
  urls = ["https://example-cookiebot.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Cookiebot Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example Cookiebot Trigger"
}

resource "matomo_tagmanager_tag_cookiebot" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-cookiebot"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  cookiebot_id = "example-value"
}
