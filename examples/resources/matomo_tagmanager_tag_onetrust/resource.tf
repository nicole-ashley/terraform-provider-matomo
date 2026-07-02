
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example OneTrust Site"
  urls = ["https://example-onetrust.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example OneTrust Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example OneTrust Trigger"
}

resource "matomo_tagmanager_tag_onetrust" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-onetrust"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  domain = "example-value"
}
