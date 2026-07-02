
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example CustomJsFunction Site"
  urls = ["https://example-customjsfunction.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example CustomJsFunction Container"
}

resource "matomo_tagmanager_variable_customjsfunction" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-customjsfunction"
  js_function = "example-value"
}
