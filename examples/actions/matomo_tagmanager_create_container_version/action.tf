action "matomo_tagmanager_create_container_version" "checkpoint" {
  config {
    container_id = matomo_tagmanager_container.main.id
    name         = "pre-release-checkpoint"
    description  = "Snapshot before publishing new changes"
  }
}
