
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Constant Site"
  urls = ["https://example-constant.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Constant Container"
}

resource "matomo_tagmanager_variable_constant" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-constant"
  constant_value = "example-value"
}
