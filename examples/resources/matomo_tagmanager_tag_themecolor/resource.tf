
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example ThemeColor Site"
  urls = ["https://example-themecolor.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example ThemeColor Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example ThemeColor Trigger"
}

resource "matomo_tagmanager_tag_themecolor" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-themecolor"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  theme_color = "example-value"
}
