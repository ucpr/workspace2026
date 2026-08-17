package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/ucpr/releascope/internal/apperr"
)

// DataStore holds raw, potentially large release artifacts (release
// notes, compare diffs, PR lists, LLM request/response payloads) in S3
// under a deterministic key layout, so Step Functions state payloads can
// carry a short S3 URI instead of the artifact itself (spec section 7,
// explicitly recommended to keep within Lambda/Step Functions payload
// limits).
type DataStore struct {
	client *s3.Client
	bucket string
}

func NewDataStore(client *s3.Client, bucket string) *DataStore {
	return &DataStore{client: client, bucket: bucket}
}

// ReleaseDataKey builds the deterministic S3 key for a piece of raw data
// belonging to one release, matching spec section 11's layout:
//
//	releases/<owner>/<repo>/<version>/<filename>
func ReleaseDataKey(owner, repo, version, filename string) string {
	return fmt.Sprintf("releases/%s/%s/%s/%s", owner, repo, version, filename)
}

// PutJSON marshals v and writes it to key, returning the resulting
// s3:// URI.
func (d *DataStore) PutJSON(ctx context.Context, key string, v any) (string, error) {
	_, span := tracer.Start(ctx, "s3.put_object", oteltrace.WithAttributes(
		attribute.String("cloud.resource_id", d.bucket),
		attribute.String("aws.s3.key", key),
	))
	defer span.End()

	body, err := json.Marshal(v)
	if err != nil {
		return "", wrapErr(ctx, apperr.New(apperr.StorageError, false, err))
	}

	_, err = d.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(d.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return "", wrapErr(ctx, apperr.New(apperr.StorageError, true, err))
	}

	span.SetAttributes(attribute.Int("aws.s3.object_size", len(body)))
	return fmt.Sprintf("s3://%s/%s", d.bucket, key), nil
}

// GetJSON reads key and unmarshals it into out.
func (d *DataStore) GetJSON(ctx context.Context, key string, out any) error {
	_, span := tracer.Start(ctx, "s3.get_object", oteltrace.WithAttributes(
		attribute.String("cloud.resource_id", d.bucket),
		attribute.String("aws.s3.key", key),
	))
	defer span.End()

	resp, err := d.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return wrapErr(ctx, apperr.New(apperr.StorageError, true, err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return wrapErr(ctx, apperr.New(apperr.StorageError, true, err))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return wrapErr(ctx, apperr.New(apperr.StorageError, false, err))
	}
	return nil
}

// GetJSONByURI is a convenience wrapper for the "s3://bucket/key" URIs
// that flow between Step Functions states.
func (d *DataStore) GetJSONByURI(ctx context.Context, uri string, out any) error {
	_, key, err := ParseS3URI(uri)
	if err != nil {
		return apperr.New(apperr.InvalidInput, false, err)
	}
	return d.GetJSON(ctx, key, out)
}

// ParseS3URI splits an "s3://bucket/key" URI into its parts.
func ParseS3URI(uri string) (bucket, key string, err error) {
	const prefix = "s3://"
	if !strings.HasPrefix(uri, prefix) {
		return "", "", fmt.Errorf("invalid s3 uri %q", uri)
	}
	rest := strings.TrimPrefix(uri, prefix)
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid s3 uri %q", uri)
	}
	return parts[0], parts[1], nil
}
