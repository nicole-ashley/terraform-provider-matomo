
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example UrlParameter Site"
  urls = ["https://example-urlparameter.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example UrlParameter Container"
}

resource "matomo_tagmanager_variable_urlparameter" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-urlparameter"
  parameter_name = "example-value"
}
