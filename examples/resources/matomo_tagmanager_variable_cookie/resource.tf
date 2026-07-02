
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Cookie Site"
  urls = ["https://example-cookie.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Cookie Container"
}

resource "matomo_tagmanager_variable_cookie" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-cookie"
  cookie_name = "example-value"
}
