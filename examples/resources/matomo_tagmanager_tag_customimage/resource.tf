
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example CustomImage Site"
  urls = ["https://example-customimage.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example CustomImage Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example CustomImage Trigger"
}

resource "matomo_tagmanager_tag_customimage" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-customimage"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  custom_image_src = "example-value"
}
