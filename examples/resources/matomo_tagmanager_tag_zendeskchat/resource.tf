
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example ZendeskChat Site"
  urls = ["https://example-zendeskchat.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example ZendeskChat Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example ZendeskChat Trigger"
}

resource "matomo_tagmanager_tag_zendeskchat" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-zendeskchat"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  zendesk_chat_id = "example-value"
}
