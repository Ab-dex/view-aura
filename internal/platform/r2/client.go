package r2

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog/log"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

const defaultPresignTTL = 15 * time.Minute

// ObjectStore is the interface satisfied by both *Client (real R2) and
// *ResilientR2Client (resilient wrapper).  The upload service and any other
// caller accepts ObjectStore so the resilient wrapper is a drop-in replacement.
type ObjectStore interface {
	PresignPutObject(ctx context.Context, key, contentType string) (string, error)
	PresignGetObject(ctx context.Context, key string) (string, error)
	DeleteObject(ctx context.Context, key string) error
	CopyObject(ctx context.Context, srcKey, dstKey string) error
}

// Client is a thin wrapper around the AWS S3 SDK pointed at Cloudflare R2.
// R2 is S3-compatible — the only difference from standard S3 usage is the
// custom endpoint URL and the "auto" region string.
type Client struct {
	svc    *s3.Client
	pre    *s3.PresignClient
	bucket string
	ttl    time.Duration
}

// Ensure *Client satisfies ObjectStore.
var _ ObjectStore = (*Client)(nil)

// New constructs an R2 client from config.
func New(cfg config.R2Config) *Client {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)

	creds := credentials.NewStaticCredentialsProvider(
		cfg.AccessKeyID,
		cfg.SecretAccessKey,
		"",
	)

	awsCfg := aws.Config{
		Region:      "auto",
		Credentials: creds,
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: endpoint}, nil
			},
		),
	}

	svc := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	ttl := cfg.PresignTTL
	if ttl == 0 {
		ttl = defaultPresignTTL
	}

	log.Info().
		Str("bucket", cfg.Bucket).
		Str("account_id", cfg.AccountID).
		Dur("presign_ttl", ttl).
		Msg("r2: client initialised")

	return &Client{
		svc:    svc,
		pre:    s3.NewPresignClient(svc),
		bucket: cfg.Bucket,
		ttl:    ttl,
	}
}

// PresignPutObject returns a presigned PUT URL for direct client upload.
func (c *Client) PresignPutObject(ctx context.Context, key, contentType string) (string, error) {
	req, err := c.pre.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(c.ttl))
	if err != nil {
		return "", fmt.Errorf("r2: presign put %s: %w", key, err)
	}
	return req.URL, nil
}

// PresignGetObject returns a presigned GET URL for time-limited downloads.
func (c *Client) PresignGetObject(ctx context.Context, key string) (string, error) {
	req, err := c.pre.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(c.ttl))
	if err != nil {
		return "", fmt.Errorf("r2: presign get %s: %w", key, err)
	}
	return req.URL, nil
}

// DeleteObject removes a stored object. Used on upload abort and moderation reject.
func (c *Client) DeleteObject(ctx context.Context, key string) error {
	_, err := c.svc.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("r2: delete %s: %w", key, err)
	}
	return nil
}

// CopyObject duplicates an object within the same bucket.
// Used by the upload pipeline to promote files from ingest to delivery prefix.
func (c *Client) CopyObject(ctx context.Context, srcKey, dstKey string) error {
	copySource := c.bucket + "/" + srcKey
	_, err := c.svc.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(c.bucket),
		CopySource: aws.String(copySource),
		Key:        aws.String(dstKey),
	})
	if err != nil {
		return fmt.Errorf("r2: copy %s → %s: %w", srcKey, dstKey, err)
	}
	return nil
}
