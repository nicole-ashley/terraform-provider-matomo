resource "matomo_tagmanager_variable" "environment" {
  container_id = matomo_tagmanager_container.main.id
  type         = "Constant"
  name         = "Environment Name"

  parameter {
    name  = "constantValue"
    value = "production"
  }
}
