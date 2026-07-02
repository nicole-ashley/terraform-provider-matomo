
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example GoogleConsentModeV2 Site"
  urls = ["https://example-googleconsentmodev2.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example GoogleConsentModeV2 Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example GoogleConsentModeV2 Trigger"
}

resource "matomo_tagmanager_tag_googleconsentmodev2" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-googleconsentmodev2"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  consent_action = ["example-value"]
}
