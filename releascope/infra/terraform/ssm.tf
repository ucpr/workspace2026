# Terraform only creates the parameter shell; the real token is set out
# of band (see README "Deploy" section) so it never lands in a .tfvars
# file or gets echoed by `terraform plan`/`apply` (spec section 25/26).
# `ignore_changes` means Terraform will not try to overwrite (or diff
# against) whatever value is actually stored.
resource "aws_ssm_parameter" "github_token" {
  name        = var.github_token_ssm_param_name
  description = "GitHub personal access token used by Releascope's GitHub API client. Set the real value with: aws ssm put-parameter --overwrite --type SecureString --name ${var.github_token_ssm_param_name} --value <token>"
  type        = "SecureString"
  value       = "placeholder-set-me"

  lifecycle {
    ignore_changes = [value]
  }
}
