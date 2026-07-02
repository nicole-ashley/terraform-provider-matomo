
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example MatomoConfiguration Site"
  urls = ["https://example-matomoconfiguration.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example MatomoConfiguration Container"
}

resource "matomo_tagmanager_variable_matomoconfiguration" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-matomoconfiguration"
  matomo_url = "example-value"
  id_site = "example-value"
}
