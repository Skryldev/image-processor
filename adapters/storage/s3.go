package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/Skryldev/image-processor/core"
	apperrors "github.com/Skryldev/image-processor/errors"
)

// S3Config holds S3 connection parameters.
type S3Config struct {
	Bucket          string
	Region          string
	Endpoint        string // optional: MinIO, localstack, etc.
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
}

// S3Client defines the minimal AWS S3 interface used by the adapter.
// This allows injection of real aws-sdk-go-v2 clients or test doubles.
type S3Client interface {
	PutObject(ctx context.Context, bucket, key string, body io.Reader, meta map[string]string) error
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	HeadObject(ctx context.Context, bucket, key string) (bool, error)
}

// S3 is the StorageAdapter backed by AWS S3 (or S3-compatible stores).
// Inject a real S3Client built with aws-sdk-go-v2 in production.
type S3 struct {
	client S3Client
	bucket string
}

// NewS3 creates an S3 adapter.  client must not be nil.
func NewS3(client S3Client, defaultBucket string) (*S3, error) {
	if client == nil {
		return nil, fmt.Errorf("s3 storage: client must not be nil")
	}
	return &S3{client: client, bucket: defaultBucket}, nil
}

func (s *S3) bucket_(key core.StorageKey) string {
	if key.Bucket != "" {
		return key.Bucket
	}
	return s.bucket
}

func (s *S3) Put(ctx context.Context, key core.StorageKey, r io.Reader, meta map[string]string) error {
	if err := ctx.Err(); err != nil {
		return apperrors.Wrap(apperrors.CategoryStorage, "s3.put", err)
	}
	if err := s.client.PutObject(ctx, s.bucket_(key), key.Path, r, meta); err != nil {
		return apperrors.Transient("s3.put", err)
	}
	return nil
}

func (s *S3) Get(ctx context.Context, key core.StorageKey) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryStorage, "s3.get", err)
	}
	rc, err := s.client.GetObject(ctx, s.bucket_(key), key.Path)
	if err != nil {
		return nil, apperrors.Transient("s3.get", err)
	}
	return rc, nil
}

func (s *S3) Delete(ctx context.Context, key core.StorageKey) error {
	if err := ctx.Err(); err != nil {
		return apperrors.Wrap(apperrors.CategoryStorage, "s3.delete", err)
	}
	return s.client.DeleteObject(ctx, s.bucket_(key), key.Path)
}

func (s *S3) Exists(ctx context.Context, key core.StorageKey) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, apperrors.Wrap(apperrors.CategoryStorage, "s3.exists", err)
	}
	return s.client.HeadObject(ctx, s.bucket_(key), key.Path)
}

// ──────────────────────────────────────────────────────────────────────────────
// Integration guide: wiring aws-sdk-go-v2
// ──────────────────────────────────────────────────────────────────────────────
//
//  import (
//      "github.com/aws/aws-sdk-go-v2/config"
//      "github.com/aws/aws-sdk-go-v2/service/s3"
//  )
//
//  func NewRealS3Client(cfg S3Config) (S3Client, error) {
//      awsCfg, _ := config.LoadDefaultConfig(context.Background(),
//          config.WithRegion(cfg.Region),
//      )
//      return &awsS3Wrapper{client: s3.NewFromConfig(awsCfg)}, nil
//  }
//
//  type awsS3Wrapper struct{ client *s3.Client }
//
//  func (w *awsS3Wrapper) PutObject(...) error { ... }
//  etc.