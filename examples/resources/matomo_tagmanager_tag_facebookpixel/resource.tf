
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example FacebookPixel Site"
  urls = ["https://example-facebookpixel.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example FacebookPixel Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example FacebookPixel Trigger"
}

resource "matomo_tagmanager_tag_facebookpixel" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-facebookpixel"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  pixel_id = "example-value"
}
