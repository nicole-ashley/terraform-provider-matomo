resource "matomo_tagmanager_trigger" "custom" {
  container_id = matomo_tagmanager_container.main.id
  type         = "PageView"
  name         = "Custom Page View Trigger"
}
