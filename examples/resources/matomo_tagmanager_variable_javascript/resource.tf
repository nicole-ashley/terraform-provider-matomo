
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example JavaScript Site"
  urls = ["https://example-javascript.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example JavaScript Container"
}

resource "matomo_tagmanager_variable_javascript" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-javascript"
  variable_name = "example-value"
}
