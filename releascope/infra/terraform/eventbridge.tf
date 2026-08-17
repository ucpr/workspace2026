# Spec section 3/4: EventBridge Scheduler drives the recurring Release
# Check workflow. traceContext is included as a static empty object so
# every Step Functions state can safely reference $.traceContext without
# a "path not found" error, whether or not there is anything real to
# extract (see internal/telemetry/propagation.go: an empty carrier is a
# defined no-op, not an error) - a scheduled run has no live caller trace
# to continue, so it always starts a fresh trace root.
resource "aws_scheduler_schedule" "release_check" {
  name       = "${var.project_name}-release-check"
  group_name = "default"

  flexible_time_window {
    mode = "OFF"
  }

  schedule_expression          = var.schedule_expression
  schedule_expression_timezone = "UTC"
  state                        = var.schedule_enabled ? "ENABLED" : "DISABLED"

  target {
    arn      = aws_sfn_state_machine.release_check.arn
    role_arn = aws_iam_role.scheduler.arn

    input = jsonencode({
      repositories = var.repositories
      traceContext = {}
    })

    retry_policy {
      maximum_retry_attempts = 3
    }
  }
}
