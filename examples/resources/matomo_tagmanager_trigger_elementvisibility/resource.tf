
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example ElementVisibility Site"
  urls = ["https://example-elementvisibility.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example ElementVisibility Container"
}

resource "matomo_tagmanager_trigger_elementvisibility" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-elementvisibility"
  selection_method = "cssSelector"
  css_selector = "example-value"
  element_id = "example-value"
  fire_trigger_when = "every"
}
