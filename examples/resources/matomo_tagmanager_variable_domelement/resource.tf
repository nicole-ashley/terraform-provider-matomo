
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example DomElement Site"
  urls = ["https://example-domelement.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example DomElement Container"
}

resource "matomo_tagmanager_variable_domelement" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-domelement"
  selection_method = "cssSelector"
  css_selector = "example-value"
  element_id = "example-value"
}
