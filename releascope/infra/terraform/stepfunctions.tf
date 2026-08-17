resource "aws_cloudwatch_log_group" "state_machine" {
  name              = "/aws/vendedlogs/states/${var.project_name}-release-check"
  retention_in_days = var.log_retention_days
}

resource "aws_sfn_state_machine" "release_check" {
  name     = "${var.project_name}-release-check"
  role_arn = aws_iam_role.state_machine.arn

  definition = templatefile("${path.module}/statemachine.asl.json.tftpl", {
    check_release_function_arn   = aws_lambda_function.check_release.arn
    fetch_context_function_arn   = aws_lambda_function.fetch_context.arn
    analyze_release_function_arn = aws_lambda_function.analyze_release.arn
    persist_release_function_arn = aws_lambda_function.persist_release.arn
    map_max_concurrency          = var.map_max_concurrency
  })

  logging_configuration {
    log_destination        = "${aws_cloudwatch_log_group.state_machine.arn}:*"
    include_execution_data = true
    level                  = "ALL"
  }

  tracing_configuration {
    enabled = false # OTel is Releascope's tracing backbone (spec section 2); AWS X-Ray is not used, keeping one tracing story instead of two.
  }
}
