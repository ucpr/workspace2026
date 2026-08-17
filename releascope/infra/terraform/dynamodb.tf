# Spec section 10. On-demand billing keeps this at near-zero cost for a
# personal/demo workload (spec section 27's cost priority).

resource "aws_dynamodb_table" "repositories" {
  name         = "${var.project_name}-repositories"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "repository"

  attribute {
    name = "repository"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }
}

resource "aws_dynamodb_table" "releases" {
  name         = "${var.project_name}-releases"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "repository"
  range_key    = "version"

  attribute {
    name = "repository"
    type = "S"
  }

  attribute {
    name = "version"
    type = "S"
  }

  attribute {
    name = "publishedAt"
    type = "S"
  }

  # release.version does not sort correctly as a string once a component
  # reaches two digits (e.g. "v0.99.0" vs "v0.100.0"), so listing a
  # repository's releases newest-first uses this GSI instead of the base
  # table's sort key (see internal/storage/dynamodb.go).
  global_secondary_index {
    name            = "repository-publishedAt-index"
    hash_key        = "repository"
    range_key       = "publishedAt"
    projection_type = "ALL"
  }

  point_in_time_recovery {
    enabled = true
  }
}
