
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Hotjar Site"
  urls = ["https://example-hotjar.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Hotjar Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example Hotjar Trigger"
}

resource "matomo_tagmanager_tag_hotjar" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-hotjar"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  hjid = "example-value"
  hjsv = 1
}
