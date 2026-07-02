
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example JavaScriptError Site"
  urls = ["https://example-javascripterror.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example JavaScriptError Container"
}

resource "matomo_tagmanager_trigger_javascripterror" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-javascripterror"
}
