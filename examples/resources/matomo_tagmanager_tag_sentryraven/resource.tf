
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example SentryRaven Site"
  urls = ["https://example-sentryraven.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example SentryRaven Container"
}

resource "matomo_tagmanager_trigger" "example" {
  container_id = matomo_tagmanager_container.example.id
  type         = "PageView"
  name         = "Example SentryRaven Trigger"
}

resource "matomo_tagmanager_tag_sentryraven" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-sentryraven"
  fire_trigger_ids = [matomo_tagmanager_trigger.example.id]
  sentry_dsn = "example-value"
}
