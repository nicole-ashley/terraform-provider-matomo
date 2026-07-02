action "matomo_tagmanager_enable_preview_mode" "preview" {
  config {
    container_id = matomo_tagmanager_container.main.id
  }
}
