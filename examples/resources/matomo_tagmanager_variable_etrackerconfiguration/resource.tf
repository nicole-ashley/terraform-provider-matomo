
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example EtrackerConfiguration Site"
  urls = ["https://example-etrackerconfiguration.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example EtrackerConfiguration Container"
}

resource "matomo_tagmanager_variable_etrackerconfiguration" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-etrackerconfiguration"
  etracker_id = "example-value"
}
