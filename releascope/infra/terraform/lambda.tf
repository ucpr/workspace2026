# Every function is built by `make build-lambdas` into bin/<name>.zip
# before `terraform apply` runs (see README's Deploy section and the
# Makefile's build-lambdas target). Using the custom `provided.al2023`
# runtime with a single "bootstrap" binary keeps each Lambda a plain Go
# binary with no framework-imposed handler shape.

locals {
  common_env = {
    AWS_REGION                  = var.aws_region
    REPOSITORIES                = join(",", var.repositories)
    REPOSITORY_TABLE_NAME       = aws_dynamodb_table.repositories.name
    RELEASE_TABLE_NAME          = aws_dynamodb_table.releases.name
    DATA_BUCKET_NAME            = aws_s3_bucket.data.bucket
    GITHUB_TOKEN_PARAM          = aws_ssm_parameter.github_token.name
    GITHUB_API_BASE_URL         = var.github_api_base_url
    BEDROCK_MODEL_ID            = var.bedrock_model_id
    OTEL_EXPORTER_OTLP_ENDPOINT = var.otel_exporter_otlp_endpoint
    ENVIRONMENT                 = var.environment
    LOG_LEVEL                   = var.log_level
  }

  lambda_zip_dir = "${path.module}/../../bin"
}

# ---- check-release ---------------------------------------------------

resource "aws_lambda_function" "check_release" {
  function_name = "${var.project_name}-check-release"
  role          = aws_iam_role.check_release.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = [var.lambda_architecture]
  timeout       = 30
  memory_size   = 256

  filename         = "${local.lambda_zip_dir}/check-release.zip"
  source_code_hash = filebase64sha256("${local.lambda_zip_dir}/check-release.zip")

  environment {
    variables = merge(local.common_env, { OTEL_SERVICE_NAME = "releascope-check-release" })
  }

  depends_on = [aws_cloudwatch_log_group.check_release]
}

resource "aws_cloudwatch_log_group" "check_release" {
  name              = "/aws/lambda/${var.project_name}-check-release"
  retention_in_days = var.log_retention_days
}

# ---- fetch-context -----------------------------------------------------

resource "aws_lambda_function" "fetch_context" {
  function_name = "${var.project_name}-fetch-context"
  role          = aws_iam_role.fetch_context.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = [var.lambda_architecture]
  timeout       = 60
  memory_size   = 256

  filename         = "${local.lambda_zip_dir}/fetch-context.zip"
  source_code_hash = filebase64sha256("${local.lambda_zip_dir}/fetch-context.zip")

  environment {
    variables = merge(local.common_env, { OTEL_SERVICE_NAME = "releascope-fetch-context" })
  }

  depends_on = [aws_cloudwatch_log_group.fetch_context]
}

resource "aws_cloudwatch_log_group" "fetch_context" {
  name              = "/aws/lambda/${var.project_name}-fetch-context"
  retention_in_days = var.log_retention_days
}

# ---- analyze-release -----------------------------------------------------

resource "aws_lambda_function" "analyze_release" {
  function_name = "${var.project_name}-analyze-release"
  role          = aws_iam_role.analyze_release.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = [var.lambda_architecture]
  timeout       = 120
  memory_size   = 512

  filename         = "${local.lambda_zip_dir}/analyze-release.zip"
  source_code_hash = filebase64sha256("${local.lambda_zip_dir}/analyze-release.zip")

  environment {
    variables = merge(local.common_env, { OTEL_SERVICE_NAME = "releascope-analyze-release" })
  }

  depends_on = [aws_cloudwatch_log_group.analyze_release]
}

resource "aws_cloudwatch_log_group" "analyze_release" {
  name              = "/aws/lambda/${var.project_name}-analyze-release"
  retention_in_days = var.log_retention_days
}

# ---- persist-release -----------------------------------------------------

resource "aws_lambda_function" "persist_release" {
  function_name = "${var.project_name}-persist-release"
  role          = aws_iam_role.persist_release.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = [var.lambda_architecture]
  timeout       = 30
  memory_size   = 256

  filename         = "${local.lambda_zip_dir}/persist-release.zip"
  source_code_hash = filebase64sha256("${local.lambda_zip_dir}/persist-release.zip")

  environment {
    variables = merge(local.common_env, { OTEL_SERVICE_NAME = "releascope-persist-release" })
  }

  depends_on = [aws_cloudwatch_log_group.persist_release]
}

resource "aws_cloudwatch_log_group" "persist_release" {
  name              = "/aws/lambda/${var.project_name}-persist-release"
  retention_in_days = var.log_retention_days
}

# ---- api -----------------------------------------------------

resource "aws_lambda_function" "api" {
  function_name = "${var.project_name}-api"
  role          = aws_iam_role.api.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = [var.lambda_architecture]
  timeout       = 30
  memory_size   = 256

  filename         = "${local.lambda_zip_dir}/api.zip"
  source_code_hash = filebase64sha256("${local.lambda_zip_dir}/api.zip")

  environment {
    variables = merge(local.common_env, {
      OTEL_SERVICE_NAME = "releascope-api"
      STATE_MACHINE_ARN = aws_sfn_state_machine.release_check.arn
    })
  }

  depends_on = [aws_cloudwatch_log_group.api]
}

resource "aws_cloudwatch_log_group" "api" {
  name              = "/aws/lambda/${var.project_name}-api"
  retention_in_days = var.log_retention_days
}
