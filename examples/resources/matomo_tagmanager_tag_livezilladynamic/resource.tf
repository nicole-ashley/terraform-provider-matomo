
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example LivezillaDynamic Site"
  urls = ["https://example-livezilladynamic.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example LivezillaDynamic Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example LivezillaDynamic Trigger"
}

resource "matomo_tagmanager_tag_livezilladynamic" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-livezilladynamic"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  livezilla_dynamic_id = "example-value"
  livezilla_dynamic_domain = "example-value"
}
