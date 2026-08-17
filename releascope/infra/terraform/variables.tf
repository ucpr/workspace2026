variable "aws_region" {
  description = "AWS region to deploy Releascope into."
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Prefix applied to every resource name."
  type        = string
  default     = "releascope"
}

variable "environment" {
  description = "Deployment environment name, used as the OTel deployment.environment resource attribute."
  type        = string
  default     = "dev"
}

variable "repositories" {
  description = "GitHub \"owner/repo\" strings Releascope tracks (spec section 1 MVP scope)."
  type        = list(string)
  default = [
    "open-telemetry/opentelemetry-collector",
    "open-telemetry/opentelemetry-collector-contrib",
  ]
}

variable "schedule_expression" {
  description = "EventBridge Scheduler expression for the recurring Release Check workflow. Default is every hour (spec section 3); change freely, e.g. \"rate(6 hours)\" or a cron(...) expression."
  type        = string
  default     = "rate(1 hour)"
}

variable "schedule_enabled" {
  description = "Whether the EventBridge Scheduler schedule is enabled. Set to false to stop scheduled polling while keeping the infrastructure (and manual-trigger API) in place."
  type        = bool
  default     = true
}

variable "map_max_concurrency" {
  description = "Step Functions Map state MaxConcurrency - how many repositories are checked in parallel per execution."
  type        = number
  default     = 5
}

variable "bedrock_model_id" {
  description = "Bedrock model ID used for release analysis. Must support tool use (structured output)."
  type        = string
  default     = "anthropic.claude-3-5-sonnet-20241022-v2:0"
}

variable "github_api_base_url" {
  description = "GitHub REST API base URL. Overridable for testing against a mock server."
  type        = string
  default     = "https://api.github.com"
}

variable "github_token_ssm_param_name" {
  description = "SSM Parameter Store name (SecureString) holding a GitHub personal access token. Terraform creates the parameter as a placeholder only - set the real value out of band (see README) so the token never enters Terraform state or version control (spec section 25/26)."
  type        = string
  default     = "/releascope/github-token"
}

variable "otel_exporter_otlp_endpoint" {
  description = "OTLP endpoint (host:port) Lambdas export traces/metrics to, e.g. an OpenTelemetry Collector's address. Empty disables OTLP export and falls back to stdout (CloudWatch Logs) - Releascope remains fully functional without a Collector (spec section 27)."
  type        = string
  default     = ""
}

variable "log_level" {
  description = "Structured log level for all Lambdas (debug|info|warn|error)."
  type        = string
  default     = "info"
}

variable "log_retention_days" {
  description = "CloudWatch Logs retention for every Lambda/Step Functions log group."
  type        = number
  default     = 14
}

variable "lambda_architecture" {
  description = "Lambda instruction set architecture. arm64 (Graviton) is cheaper per spec section 27's cost priority."
  type        = string
  default     = "arm64"
}

variable "api_stage_name" {
  description = "API Gateway deployment stage name."
  type        = string
  default     = "v1"
}

variable "data_bucket_force_destroy" {
  description = "Allow `terraform destroy` to delete the S3 data bucket even if it still contains objects. Convenient for a throwaway dev/demo environment; leave false for anything you want to keep."
  type        = bool
  default     = false
}
