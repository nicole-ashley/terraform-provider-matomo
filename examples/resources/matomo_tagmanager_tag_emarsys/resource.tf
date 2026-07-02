
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Emarsys Site"
  urls = ["https://example-emarsys.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Emarsys Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example Emarsys Trigger"
}

resource "matomo_tagmanager_tag_emarsys" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-emarsys"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  merchant_id = "example-value"
}
