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

// Client is a thin wrapper around the AWS S3 SDK pointed at Cloudflare R2.
// R2 is S3-compatible — the only difference from standard S3 usage is the
// custom endpoint URL and the "auto" region string.
type Client struct {
	svc    *s3.Client
	pre    *s3.PresignClient
	bucket string
	ttl    time.Duration
}

// New constructs an R2 client from config.
// A connectivity check is not performed at construction — the first
// presign or object operation will surface any credential errors.
func New(cfg config.R2Config) *Client {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)

	creds := credentials.NewStaticCredentialsProvider(
		cfg.AccessKeyID,
		cfg.SecretAccessKey,
		"", // session token — not used with R2
	)

	awsCfg := aws.Config{
		Region:      "auto", // R2 requires this literal string
		Credentials: creds,
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: endpoint}, nil
			},
		),
	}

	svc := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		// R2 requires path-style addressing: endpoint/bucket/key
		// rather than bucket.endpoint/key (virtual-hosted style).
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

// PresignPutObject returns a pre-signed URL the client uses to upload a file
// directly to R2 — bypassing the API server entirely.
// The URL expires after Client.ttl (default 15 minutes).
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

// PresignGetObject returns a pre-signed URL for a time-limited download
// of a private object (e.g. a screener that requires a license).
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

// DeleteObject removes a stored object.
// Used by the upload module when an upload is aborted and by the moderation
// module when a rejected asset must be permanently purged.
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
// Used by the upload pipeline to promote a file from the ingest prefix
// (uploads/) to the delivery prefix (assets/) after transcoding is complete.
func (c *Client) CopyObject(ctx context.Context, srcKey, dstKey string) error {
	// R2 copy source must be bucket/key.
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
