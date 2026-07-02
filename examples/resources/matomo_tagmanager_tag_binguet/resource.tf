
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example BingUET Site"
  urls = ["https://example-binguet.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example BingUET Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example BingUET Trigger"
}

resource "matomo_tagmanager_tag_binguet" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-binguet"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  bing_ad_id = "example-value"
}
