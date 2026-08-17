output "api_base_url" {
  description = "Base URL of the Releascope REST API."
  value       = aws_api_gateway_stage.api.invoke_url
}

output "state_machine_arn" {
  description = "ARN of the Release Check Step Functions state machine."
  value       = aws_sfn_state_machine.release_check.arn
}

output "data_bucket_name" {
  value = aws_s3_bucket.data.bucket
}

output "repositories_table_name" {
  value = aws_dynamodb_table.repositories.name
}

output "releases_table_name" {
  value = aws_dynamodb_table.releases.name
}

output "github_token_parameter_name" {
  description = "SSM Parameter Store name to set the real GitHub token into after apply (see README)."
  value       = aws_ssm_parameter.github_token.name
}

output "invoke_manual_check_policy_arn" {
  description = "Attach this IAM policy to any principal that needs to call POST /repositories/{owner}/{repo}/check."
  value       = aws_iam_policy.invoke_manual_check.arn
}
