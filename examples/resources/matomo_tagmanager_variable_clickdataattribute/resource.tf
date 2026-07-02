
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example ClickDataAttribute Site"
  urls = ["https://example-clickdataattribute.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example ClickDataAttribute Container"
}

resource "matomo_tagmanager_variable_clickdataattribute" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-clickdataattribute"
  data_attribute = "example-value"
}
