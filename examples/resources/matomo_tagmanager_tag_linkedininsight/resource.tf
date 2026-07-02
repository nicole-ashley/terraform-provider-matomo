
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example LinkedinInsight Site"
  urls = ["https://example-linkedininsight.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example LinkedinInsight Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example LinkedinInsight Trigger"
}

resource "matomo_tagmanager_tag_linkedininsight" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-linkedininsight"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  partner_id = "example-value"
}
