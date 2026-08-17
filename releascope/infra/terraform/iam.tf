# Least-privilege IAM: one role per Lambda function, each granted only
# the table/bucket/model access that function actually needs (spec
# section 26).

data "aws_iam_policy_document" "lambda_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "lambda_logging" {
  statement {
    sid       = "WriteLogs"
    actions   = ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["arn:aws:logs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:log-group:/aws/lambda/${var.project_name}-*:*"]
  }
}

data "aws_iam_policy_document" "read_github_token" {
  statement {
    sid       = "ReadGitHubToken"
    actions   = ["ssm:GetParameter"]
    resources = [aws_ssm_parameter.github_token.arn]
  }
}

# ---- check-release ---------------------------------------------------

resource "aws_iam_role" "check_release" {
  name               = "${var.project_name}-check-release"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

data "aws_iam_policy_document" "check_release" {
  statement {
    sid       = "ReadRepositoryPointer"
    actions   = ["dynamodb:GetItem", "dynamodb:UpdateItem"]
    resources = [aws_dynamodb_table.repositories.arn]
  }
  statement {
    sid       = "WriteRawReleaseSnapshot"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.data.arn}/releases/*"]
  }
}

resource "aws_iam_role_policy" "check_release" {
  name   = "access"
  role   = aws_iam_role.check_release.id
  policy = data.aws_iam_policy_document.check_release.json
}

resource "aws_iam_role_policy" "check_release_logging" {
  name   = "logging"
  role   = aws_iam_role.check_release.id
  policy = data.aws_iam_policy_document.lambda_logging.json
}

resource "aws_iam_role_policy" "check_release_github_token" {
  name   = "github-token"
  role   = aws_iam_role.check_release.id
  policy = data.aws_iam_policy_document.read_github_token.json
}

# ---- fetch-context -----------------------------------------------------

resource "aws_iam_role" "fetch_context" {
  name               = "${var.project_name}-fetch-context"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

data "aws_iam_policy_document" "fetch_context" {
  statement {
    sid       = "ReadWriteRawContext"
    actions   = ["s3:GetObject", "s3:PutObject"]
    resources = ["${aws_s3_bucket.data.arn}/releases/*"]
  }
}

resource "aws_iam_role_policy" "fetch_context" {
  name   = "access"
  role   = aws_iam_role.fetch_context.id
  policy = data.aws_iam_policy_document.fetch_context.json
}

resource "aws_iam_role_policy" "fetch_context_logging" {
  name   = "logging"
  role   = aws_iam_role.fetch_context.id
  policy = data.aws_iam_policy_document.lambda_logging.json
}

resource "aws_iam_role_policy" "fetch_context_github_token" {
  name   = "github-token"
  role   = aws_iam_role.fetch_context.id
  policy = data.aws_iam_policy_document.read_github_token.json
}

# ---- analyze-release -----------------------------------------------------

resource "aws_iam_role" "analyze_release" {
  name               = "${var.project_name}-analyze-release"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

data "aws_iam_policy_document" "analyze_release" {
  statement {
    sid       = "ReadWriteRawAnalysis"
    actions   = ["s3:GetObject", "s3:PutObject"]
    resources = ["${aws_s3_bucket.data.arn}/releases/*"]
  }
  statement {
    sid     = "InvokeBedrockModel"
    actions = ["bedrock:InvokeModel"]
    resources = [
      "arn:aws:bedrock:${data.aws_region.current.name}::foundation-model/${var.bedrock_model_id}",
    ]
  }
}

resource "aws_iam_role_policy" "analyze_release" {
  name   = "access"
  role   = aws_iam_role.analyze_release.id
  policy = data.aws_iam_policy_document.analyze_release.json
}

resource "aws_iam_role_policy" "analyze_release_logging" {
  name   = "logging"
  role   = aws_iam_role.analyze_release.id
  policy = data.aws_iam_policy_document.lambda_logging.json
}

# ---- persist-release -----------------------------------------------------

resource "aws_iam_role" "persist_release" {
  name               = "${var.project_name}-persist-release"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

data "aws_iam_policy_document" "persist_release" {
  statement {
    sid = "TransactionalWrite"
    actions = [
      "dynamodb:PutItem",
      "dynamodb:UpdateItem",
      "dynamodb:TransactWriteItems",
    ]
    resources = [
      aws_dynamodb_table.releases.arn,
      aws_dynamodb_table.repositories.arn,
    ]
  }
}

resource "aws_iam_role_policy" "persist_release" {
  name   = "access"
  role   = aws_iam_role.persist_release.id
  policy = data.aws_iam_policy_document.persist_release.json
}

resource "aws_iam_role_policy" "persist_release_logging" {
  name   = "logging"
  role   = aws_iam_role.persist_release.id
  policy = data.aws_iam_policy_document.lambda_logging.json
}

# ---- api -----------------------------------------------------

resource "aws_iam_role" "api" {
  name               = "${var.project_name}-api"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

data "aws_iam_policy_document" "api" {
  statement {
    sid       = "ReadReleaseData"
    actions   = ["dynamodb:GetItem", "dynamodb:Query"]
    resources = [
      aws_dynamodb_table.repositories.arn,
      aws_dynamodb_table.releases.arn,
      "${aws_dynamodb_table.releases.arn}/index/*",
    ]
  }
  statement {
    sid       = "TriggerManualCheck"
    actions   = ["states:StartExecution"]
    resources = [aws_sfn_state_machine.release_check.arn]
  }
}

resource "aws_iam_role_policy" "api" {
  name   = "access"
  role   = aws_iam_role.api.id
  policy = data.aws_iam_policy_document.api.json
}

resource "aws_iam_role_policy" "api_logging" {
  name   = "logging"
  role   = aws_iam_role.api.id
  policy = data.aws_iam_policy_document.lambda_logging.json
}

# ---- Step Functions execution role -----------------------------------

resource "aws_iam_role" "state_machine" {
  name = "${var.project_name}-state-machine"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "states.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

data "aws_iam_policy_document" "state_machine" {
  statement {
    sid     = "InvokeTaskLambdas"
    actions = ["lambda:InvokeFunction"]
    resources = [
      aws_lambda_function.check_release.arn,
      aws_lambda_function.fetch_context.arn,
      aws_lambda_function.analyze_release.arn,
      aws_lambda_function.persist_release.arn,
    ]
  }
  statement {
    sid = "ExecutionLogging"
    actions = [
      "logs:CreateLogDelivery",
      "logs:GetLogDelivery",
      "logs:UpdateLogDelivery",
      "logs:DeleteLogDelivery",
      "logs:ListLogDeliveries",
      "logs:PutResourcePolicy",
      "logs:DescribeResourcePolicies",
      "logs:DescribeLogGroups",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "state_machine" {
  name   = "access"
  role   = aws_iam_role.state_machine.id
  policy = data.aws_iam_policy_document.state_machine.json
}

# ---- EventBridge Scheduler role ---------------------------------------

resource "aws_iam_role" "scheduler" {
  name = "${var.project_name}-scheduler"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "scheduler.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

data "aws_iam_policy_document" "scheduler" {
  statement {
    sid       = "StartReleaseCheck"
    actions   = ["states:StartExecution"]
    resources = [aws_sfn_state_machine.release_check.arn]
  }
}

resource "aws_iam_role_policy" "scheduler" {
  name   = "access"
  role   = aws_iam_role.scheduler.id
  policy = data.aws_iam_policy_document.scheduler.json
}
