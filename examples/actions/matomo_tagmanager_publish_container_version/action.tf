action "matomo_tagmanager_publish_container_version" "go_live" {
  config {
    container_id = matomo_tagmanager_container.main.id
    environment  = "live"
  }
}
