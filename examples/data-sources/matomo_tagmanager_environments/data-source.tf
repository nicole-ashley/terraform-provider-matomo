data "matomo_tagmanager_environments" "all" {}

output "available_environments" {
  value = [for e in data.matomo_tagmanager_environments.all.environments : e.id]
}
