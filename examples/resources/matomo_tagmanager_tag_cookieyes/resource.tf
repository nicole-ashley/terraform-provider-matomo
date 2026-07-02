
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example CookieYes Site"
  urls = ["https://example-cookieyes.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example CookieYes Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example CookieYes Trigger"
}

resource "matomo_tagmanager_tag_cookieyes" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-cookieyes"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  cookie_yes_website_key = "example-value"
}
