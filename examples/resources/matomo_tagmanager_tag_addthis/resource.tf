
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example AddThis Site"
  urls = ["https://example-addthis.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example AddThis Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example AddThis Trigger"
}

resource "matomo_tagmanager_tag_addthis" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-addthis"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  add_this_pub_id = "example-value"
}
