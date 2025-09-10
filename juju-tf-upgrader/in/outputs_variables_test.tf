# Test file for output and variable transformations
data "juju_model" "production" {
  name = "production-env"
}

resource "juju_model" "development" {
  name = "dev-environment"
}

# Outputs that should be upgraded
output "database_model" {
  value = juju_model.development.name
}

output "monitoring_model" {
  value = data.juju_model.production.name
}

# Output that should NOT be upgraded (not referencing .name)
output "model_id" {
  value = juju_model.development.id
}

# Output that should NOT be upgraded (variable reference)
output "var_model" {
  value = var.model_name
}

# Variables that should trigger warnings
variable "default_model" {
  description = "The default model name to use"
  type        = string
  default     = "default"
}

variable "model_name" {
  description = "Name of the model"
  type        = string
}

variable "model_uuid_var" {
  description = "UUID of the model"
  type        = string
}

# Variable that should NOT trigger warning (no 'model' in name)
variable "application_name" {
  description = "Name of the application"
  type        = string
}
