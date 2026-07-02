
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Raygun Site"
  urls = ["https://example-raygun.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Raygun Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example Raygun Trigger"
}

resource "matomo_tagmanager_tag_raygun" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-raygun"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  raygun_api_key = "example-value"
}
