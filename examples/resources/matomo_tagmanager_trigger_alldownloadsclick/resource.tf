
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example AllDownloadsClick Site"
  urls = ["https://example-alldownloadsclick.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example AllDownloadsClick Container"
}

resource "matomo_tagmanager_trigger_alldownloadsclick" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-alldownloadsclick"
  download_extensions = "example-value"
}
