resource "matomo_tagmanager_trigger" "all_pages" {
  container_id = matomo_tagmanager_container.main.id
  type         = "PageView"
  name         = "All Page Views"
}

resource "matomo_tagmanager_tag" "custom_html" {
  container_id     = matomo_tagmanager_container.main.id
  type             = "CustomHtml"
  name             = "Custom HTML Tag"
  fire_trigger_ids = [matomo_tagmanager_trigger.all_pages.id]

  parameter {
    name  = "customHtml"
    value = "<script>console.log('loaded');</script>"
  }
}
