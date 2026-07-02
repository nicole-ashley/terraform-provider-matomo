
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example Etracker Site"
  urls = ["https://example-etracker.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example Etracker Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example Etracker Trigger"
}

resource "matomo_tagmanager_tag_etracker" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-etracker"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  tracking_type = "addtocart"
  etracker_add_to_cart_product = "example-value"
}
