
provider "matomo" {}

resource "matomo_site" "example" {
  name = "Example ClickHtmlAttribute Site"
  urls = ["https://example-clickhtmlattribute.example.com"]
}

resource "matomo_tagmanager_container" "example" {
  site_id = matomo_site.example.id
  context = "web"
  name    = "Example ClickHtmlAttribute Container"
}

resource "matomo_tagmanager_variable_clickhtmlattribute" "example" {
  container_id = matomo_tagmanager_container.example.id
  name         = "example-clickhtmlattribute"
  html_attribute = "example-value"
}
