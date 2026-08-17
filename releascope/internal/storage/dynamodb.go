// Package storage implements DynamoDB and S3 persistence for
// Releascope. It is the only package that knows about table/bucket
// layout; callers work with plain internal/release domain types.
package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/ucpr/releascope/internal/apperr"
	"github.com/ucpr/releascope/internal/fault"
	"github.com/ucpr/releascope/internal/release"
)

var tracer = otel.Tracer("releascope/internal/storage")

// PublishedAtIndex is the GSI name used to list a repository's releases
// ordered by publish date (release.version does not sort correctly as a
// string once a component reaches two digits, e.g. "v0.99.0" vs
// "v0.100.0", so listing uses this GSI instead of the base table's sort
// key). See infra/terraform/dynamodb.tf.
const PublishedAtIndex = "repository-publishedAt-index"

// RepositoryStore accesses the "repositories" table.
type RepositoryStore struct {
	client *dynamodb.Client
	table  string
}

func NewRepositoryStore(client *dynamodb.Client, table string) *RepositoryStore {
	return &RepositoryStore{client: client, table: table}
}

// Get returns the stored repository pointer, or (nil, nil) if the
// repository has never been checked before - a first-time check is not
// an error.
func (s *RepositoryStore) Get(ctx context.Context, repository string) (*release.Repository, error) {
	ctx, span := tracer.Start(ctx, "dynamodb.get_item", oteltrace.WithAttributes(
		attribute.String("db.system", "dynamodb"),
		attribute.String("db.collection.name", s.table),
		attribute.String("release.repository", repository),
	))
	defer span.End()

	if err := fault.MaybeInject(ctx, fault.DynamoDBError, 0); err != nil {
		return nil, wrapErr(ctx, err)
	}

	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"repository": &types.AttributeValueMemberS{Value: repository},
		},
	})
	if err != nil {
		return nil, wrapErr(ctx, err)
	}
	if out.Item == nil {
		return nil, nil
	}

	var repo release.Repository
	if err := attributevalue.UnmarshalMap(out.Item, &repo); err != nil {
		return nil, wrapErr(ctx, err)
	}
	return &repo, nil
}

// MarkChecked records that a check happened without finding a new
// release (spec Scenario 002). It is a plain attribute update, safe to
// call repeatedly (idempotent).
func (s *RepositoryStore) MarkChecked(ctx context.Context, repository string, checkedAt time.Time) error {
	ctx, span := tracer.Start(ctx, "dynamodb.update_item", oteltrace.WithAttributes(
		attribute.String("db.system", "dynamodb"),
		attribute.String("db.collection.name", s.table),
		attribute.String("release.repository", repository),
	))
	defer span.End()

	if err := fault.MaybeInject(ctx, fault.DynamoDBError, 0); err != nil {
		return wrapErr(ctx, err)
	}

	now := checkedAt.Format(time.RFC3339Nano)
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			"repository": &types.AttributeValueMemberS{Value: repository},
		},
		UpdateExpression: aws.String("SET lastCheckedAt = :now, createdAt = if_not_exists(createdAt, :now)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now": &types.AttributeValueMemberS{Value: now},
		},
	})
	if err != nil {
		return wrapErr(ctx, err)
	}
	return nil
}

// ReleaseStore accesses the "releases" table plus the repositories
// pointer it must stay consistent with.
type ReleaseStore struct {
	client          *dynamodb.Client
	releaseTable    string
	repositoryTable string
}

func NewReleaseStore(client *dynamodb.Client, releaseTable, repositoryTable string) *ReleaseStore {
	return &ReleaseStore{client: client, releaseTable: releaseTable, repositoryTable: repositoryTable}
}

// Get fetches a single release by repository+version.
func (s *ReleaseStore) Get(ctx context.Context, repository, version string) (*release.Record, error) {
	ctx, span := tracer.Start(ctx, "dynamodb.get_item", oteltrace.WithAttributes(
		attribute.String("db.system", "dynamodb"),
		attribute.String("db.collection.name", s.releaseTable),
		attribute.String("release.repository", repository),
		attribute.String("release.version", version),
	))
	defer span.End()

	if err := fault.MaybeInject(ctx, fault.DynamoDBError, 0); err != nil {
		return nil, wrapErr(ctx, err)
	}

	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.releaseTable),
		Key: map[string]types.AttributeValue{
			"repository": &types.AttributeValueMemberS{Value: repository},
			"version":    &types.AttributeValueMemberS{Value: version},
		},
	})
	if err != nil {
		return nil, wrapErr(ctx, err)
	}
	if out.Item == nil {
		return nil, nil
	}

	var rec release.Record
	if err := attributevalue.UnmarshalMap(out.Item, &rec); err != nil {
		return nil, wrapErr(ctx, err)
	}
	return &rec, nil
}

// List returns releases for a repository ordered newest-first by
// publish date, paginated via an opaque cursor token.
func (s *ReleaseStore) List(ctx context.Context, repository string, limit int32, cursor string) (records []release.Record, nextCursor string, err error) {
	ctx, span := tracer.Start(ctx, "dynamodb.query", oteltrace.WithAttributes(
		attribute.String("db.system", "dynamodb"),
		attribute.String("db.collection.name", s.releaseTable),
		attribute.String("release.repository", repository),
	))
	defer span.End()

	if err := fault.MaybeInject(ctx, fault.DynamoDBError, 0); err != nil {
		return nil, "", wrapErr(ctx, err)
	}

	input := &dynamodb.QueryInput{
		TableName:              aws.String(s.releaseTable),
		IndexName:              aws.String(PublishedAtIndex),
		KeyConditionExpression: aws.String("repository = :repo"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":repo": &types.AttributeValueMemberS{Value: repository},
		},
		ScanIndexForward: aws.Bool(false), // newest first
		Limit:            aws.Int32(limit),
	}
	if cursor != "" {
		startKey, decodeErr := decodeCursor(cursor)
		if decodeErr != nil {
			return nil, "", apperr.New(apperr.InvalidInput, false, decodeErr)
		}
		input.ExclusiveStartKey = startKey
	}

	out, err := s.client.Query(ctx, input)
	if err != nil {
		return nil, "", wrapErr(ctx, err)
	}

	records = make([]release.Record, 0, len(out.Items))
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &records); err != nil {
		return nil, "", wrapErr(ctx, err)
	}

	if len(out.LastEvaluatedKey) > 0 {
		nextCursor, err = encodeCursor(out.LastEvaluatedKey)
		if err != nil {
			return nil, "", wrapErr(ctx, err)
		}
	}

	span.SetAttributes(attribute.Int("release.list.count", len(records)))
	return records, nextCursor, nil
}

// Persist atomically writes a release record and advances the owning
// repository's latest-version pointer using a DynamoDB transaction, so
// a reader never observes a release row without the pointer reflecting
// it (or vice versa). Both writes are unconditional overwrites keyed by
// deterministic values (repository+version, repository), so replaying
// the exact same release is idempotent by construction (spec section 6):
// re-running this with identical input produces identical items.
func (s *ReleaseStore) Persist(ctx context.Context, rec release.Record, now time.Time) error {
	ctx, span := tracer.Start(ctx, "release.persist", oteltrace.WithAttributes(
		attribute.String("release.repository", rec.Repository),
		attribute.String("release.version", rec.Version),
		attribute.String("release.previous_version", rec.PreviousVersion),
		attribute.Bool("release.new", true),
		attribute.String("release.risk", string(rec.Risk)),
	))
	defer span.End()

	if err := fault.MaybeInject(ctx, fault.DynamoDBError, 0); err != nil {
		return wrapErr(ctx, err)
	}

	rec.CreatedAt = now
	releaseItem, err := attributevalue.MarshalMap(rec)
	if err != nil {
		return wrapErr(ctx, err)
	}

	nowStr := now.Format(time.RFC3339Nano)
	publishedAtStr := rec.PublishedAt.Format(time.RFC3339Nano)

	_, err = s.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				Put: &types.Put{
					TableName: aws.String(s.releaseTable),
					Item:      releaseItem,
				},
			},
			{
				Update: &types.Update{
					TableName: aws.String(s.repositoryTable),
					Key: map[string]types.AttributeValue{
						"repository": &types.AttributeValueMemberS{Value: rec.Repository},
					},
					UpdateExpression: aws.String(
						"SET latestVersion = :v, latestPublishedAt = :p, lastCheckedAt = :now, updatedAt = :now, createdAt = if_not_exists(createdAt, :now)",
					),
					ExpressionAttributeValues: map[string]types.AttributeValue{
						":v":   &types.AttributeValueMemberS{Value: rec.Version},
						":p":   &types.AttributeValueMemberS{Value: publishedAtStr},
						":now": &types.AttributeValueMemberS{Value: nowStr},
					},
				},
			},
		},
	})
	if err != nil {
		return wrapErr(ctx, err)
	}
	return nil
}

func wrapErr(ctx context.Context, err error) error {
	wrapped := apperr.New(apperr.StorageError, true, err)
	apperr.RecordSpanError(ctx, wrapped)
	return wrapped
}

func encodeCursor(key map[string]types.AttributeValue) (string, error) {
	m := map[string]any{}
	if err := attributevalue.UnmarshalMap(key, &m); err != nil {
		return "", err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func decodeCursor(cursor string) (map[string]types.AttributeValue, error) {
	b, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("invalid cursor: %w", err)
	}
	av, err := attributevalue.MarshalMap(m)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor: %w", err)
	}
	return av, nil
}
