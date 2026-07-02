
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example CustomHtml Site"
  urls = ["https://example-customhtml.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example CustomHtml Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example CustomHtml Trigger"
}

resource "matomo_tagmanager_tag_customhtml" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-customhtml"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  custom_html = "example-value"
}
