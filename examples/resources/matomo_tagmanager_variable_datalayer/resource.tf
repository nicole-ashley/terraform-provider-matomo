
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example DataLayer Site"
  urls = ["https://example-datalayer.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example DataLayer Container"
}

resource "matomo_tagmanager_variable_datalayer" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-datalayer"
  data_layer_name = "example-value"
}
