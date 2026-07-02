
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Url Site"
  urls = ["https://example-url.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Url Container"
}

resource "matomo_tagmanager_variable_url" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-url"
  url_part = "hash"
}
