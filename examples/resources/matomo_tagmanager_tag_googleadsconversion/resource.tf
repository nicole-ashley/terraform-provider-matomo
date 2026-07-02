
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example GoogleAdsConversion Site"
  urls = ["https://example-googleadsconversion.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example GoogleAdsConversion Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example GoogleAdsConversion Trigger"
}

resource "matomo_tagmanager_tag_googleadsconversion" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-googleadsconversion"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  google_ads_conversion_id = "example-value"
  google_ads_conversion_label = "example-value"
}
