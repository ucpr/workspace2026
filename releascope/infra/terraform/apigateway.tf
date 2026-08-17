# Spec section 12/13. Explicit resources (not a {proxy+} catch-all) so
# API Gateway hands the Lambda already-parsed path parameters
# (owner/repo/version) via events.APIGatewayProxyRequest.PathParameters.
#
# GET routes are public/unauthenticated (read-only release intelligence
# data). POST .../check is AWS_IAM-authorized (spec section 13: "本 API
# は認証なしで public に公開しないこと") - callers must sign requests
# with SigV4 using credentials that have execute-api:Invoke on this
# route (see the invoke_manual_check IAM policy output below).

resource "aws_api_gateway_rest_api" "api" {
  name = "${var.project_name}-api"

  endpoint_configuration {
    types = ["REGIONAL"]
  }
}

resource "aws_api_gateway_resource" "repositories" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_rest_api.api.root_resource_id
  path_part   = "repositories"
}

resource "aws_api_gateway_resource" "owner" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_resource.repositories.id
  path_part   = "{owner}"
}

resource "aws_api_gateway_resource" "repo" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_resource.owner.id
  path_part   = "{repo}"
}

resource "aws_api_gateway_resource" "releases" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_resource.repo.id
  path_part   = "releases"
}

resource "aws_api_gateway_resource" "version" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_resource.releases.id
  path_part   = "{version}"
}

resource "aws_api_gateway_resource" "check" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_resource.repo.id
  path_part   = "check"
}

# ---- GET /repositories ------------------------------------------------

resource "aws_api_gateway_method" "list_repositories" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  resource_id   = aws_api_gateway_resource.repositories.id
  http_method   = "GET"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "list_repositories" {
  rest_api_id             = aws_api_gateway_rest_api.api.id
  resource_id             = aws_api_gateway_resource.repositories.id
  http_method             = aws_api_gateway_method.list_repositories.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.api.invoke_arn
}

# ---- GET /repositories/{owner}/{repo}/releases -------------------------

resource "aws_api_gateway_method" "list_releases" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  resource_id   = aws_api_gateway_resource.releases.id
  http_method   = "GET"
  authorization = "NONE"

  request_parameters = {
    "method.request.path.owner" = true
    "method.request.path.repo"  = true
  }
}

resource "aws_api_gateway_integration" "list_releases" {
  rest_api_id             = aws_api_gateway_rest_api.api.id
  resource_id             = aws_api_gateway_resource.releases.id
  http_method             = aws_api_gateway_method.list_releases.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.api.invoke_arn
}

# ---- GET /repositories/{owner}/{repo}/releases/{version} ----------------

resource "aws_api_gateway_method" "get_release" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  resource_id   = aws_api_gateway_resource.version.id
  http_method   = "GET"
  authorization = "NONE"

  request_parameters = {
    "method.request.path.owner"   = true
    "method.request.path.repo"    = true
    "method.request.path.version" = true
  }
}

resource "aws_api_gateway_integration" "get_release" {
  rest_api_id             = aws_api_gateway_rest_api.api.id
  resource_id             = aws_api_gateway_resource.version.id
  http_method             = aws_api_gateway_method.get_release.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.api.invoke_arn
}

# ---- POST /repositories/{owner}/{repo}/check (IAM-authorized) ----------

resource "aws_api_gateway_method" "trigger_check" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  resource_id   = aws_api_gateway_resource.check.id
  http_method   = "POST"
  authorization = "AWS_IAM"

  request_parameters = {
    "method.request.path.owner" = true
    "method.request.path.repo"  = true
  }
}

resource "aws_api_gateway_integration" "trigger_check" {
  rest_api_id             = aws_api_gateway_rest_api.api.id
  resource_id             = aws_api_gateway_resource.check.id
  http_method             = aws_api_gateway_method.trigger_check.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.api.invoke_arn
}

resource "aws_lambda_permission" "apigateway" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.api.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.api.execution_arn}/*/*"
}

resource "aws_api_gateway_deployment" "api" {
  rest_api_id = aws_api_gateway_rest_api.api.id

  triggers = {
    redeployment = sha1(jsonencode([
      aws_api_gateway_resource.repositories.id,
      aws_api_gateway_resource.owner.id,
      aws_api_gateway_resource.repo.id,
      aws_api_gateway_resource.releases.id,
      aws_api_gateway_resource.version.id,
      aws_api_gateway_resource.check.id,
      aws_api_gateway_method.list_repositories.id,
      aws_api_gateway_method.list_releases.id,
      aws_api_gateway_method.get_release.id,
      aws_api_gateway_method.trigger_check.id,
      aws_api_gateway_integration.list_repositories.id,
      aws_api_gateway_integration.list_releases.id,
      aws_api_gateway_integration.get_release.id,
      aws_api_gateway_integration.trigger_check.id,
    ]))
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_cloudwatch_log_group" "api_gateway_access_logs" {
  name              = "/aws/apigateway/${var.project_name}-api"
  retention_in_days = var.log_retention_days
}

resource "aws_api_gateway_stage" "api" {
  deployment_id = aws_api_gateway_deployment.api.id
  rest_api_id   = aws_api_gateway_rest_api.api.id
  stage_name    = var.api_stage_name

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.api_gateway_access_logs.arn
    format = jsonencode({
      requestId      = "$context.requestId"
      ip             = "$context.identity.sourceIp"
      requestTime    = "$context.requestTime"
      httpMethod     = "$context.httpMethod"
      resourcePath   = "$context.resourcePath"
      status         = "$context.status"
      responseLength = "$context.responseLength"
      traceparent    = "$context.requestOverride.header.traceparent"
    })
  }
}

# A caller (human or automation) must be granted this policy to invoke
# the manual-trigger endpoint, e.g. via `aws sts assume-role` +
# SigV4-signed curl, or the AWS CLI's `aws apigateway test-invoke-method`
# for a quick local check.
data "aws_iam_policy_document" "invoke_manual_check" {
  statement {
    actions   = ["execute-api:Invoke"]
    resources = ["${aws_api_gateway_rest_api.api.execution_arn}/${var.api_stage_name}/POST/repositories/*/*/check"]
  }
}

resource "aws_iam_policy" "invoke_manual_check" {
  name   = "${var.project_name}-invoke-manual-check"
  policy = data.aws_iam_policy_document.invoke_manual_check.json
}
