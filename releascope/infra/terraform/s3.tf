# Spec section 11/26: raw release/compare/PR/LLM payloads, never
# exposed directly to API clients (the API only ever returns data read
# out of DynamoDB - see internal/apihandlers).

resource "aws_s3_bucket" "data" {
  bucket        = "${var.project_name}-data-${data.aws_caller_identity.current.account_id}-${data.aws_region.current.name}"
  force_destroy = var.data_bucket_force_destroy
}

resource "aws_s3_bucket_public_access_block" "data" {
  bucket = aws_s3_bucket.data.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "data" {
  bucket = aws_s3_bucket.data.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "data" {
  bucket = aws_s3_bucket.data.id

  rule {
    id     = "expire-old-raw-data"
    status = "Enabled"

    filter {}

    # Raw release context is a debugging/analysis aid, not the source of
    # truth (DynamoDB is) - expiring it after a year keeps storage cost
    # bounded without losing anything that matters long-term.
    expiration {
      days = 365
    }

    noncurrent_version_expiration {
      noncurrent_days = 30
    }
  }
}

resource "aws_s3_bucket_versioning" "data" {
  bucket = aws_s3_bucket.data.id
  versioning_configuration {
    status = "Enabled"
  }
}
