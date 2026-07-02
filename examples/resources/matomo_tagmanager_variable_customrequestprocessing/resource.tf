
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example CustomRequestProcessing Site"
  urls = ["https://example-customrequestprocessing.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example CustomRequestProcessing Container"
}

resource "matomo_tagmanager_variable_customrequestprocessing" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-customrequestprocessing"
  js_function = "example-value"
}
