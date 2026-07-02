
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example MetaContent Site"
  urls = ["https://example-metacontent.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example MetaContent Container"
}

resource "matomo_tagmanager_variable_metacontent" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-metacontent"
  meta_name = "application-name"
}
