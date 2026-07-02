
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example FormSubmit Site"
  urls = ["https://example-formsubmit.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example FormSubmit Container"
}

resource "matomo_tagmanager_trigger_formsubmit" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-formsubmit"
}
