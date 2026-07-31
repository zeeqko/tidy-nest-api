package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Config is everything needed to reach the bucket Terraform created in
// infra/. The account ID forms the S3-compatible endpoint; the key pair comes
// from R2 -> Manage API Tokens in the Cloudflare dashboard.
type R2Config struct {
	AccountID       string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
}

type r2Store struct {
	client *s3.Client
	bucket string
}

func NewR2(ctx context.Context, c R2Config) (Store, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		// R2 is region-less; the SigV4 signer still needs a region string.
		config.WithRegion("auto"),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(c.AccessKeyID, c.SecretAccessKey, ""),
		),
		// R2 rejects the trailing/streaming checksums the AWS SDK adds by
		// default, so only send one when an operation actually requires it.
		config.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		config.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://" + c.AccountID + ".r2.cloudflarestorage.com")
		// Virtual-host style would put the bucket in the hostname, which R2's
		// S3 endpoint does not serve.
		o.UsePathStyle = true
	})
	return &r2Store{client: client, bucket: c.Bucket}, nil
}

func (s *r2Store) Describe() string {
	return "Cloudflare R2 bucket " + s.bucket
}

func (s *r2Store) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

func (s *r2Store) Get(ctx context.Context, key string) (*Object, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var respErr *awshttp.ResponseError
		if errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get %s: %w", key, err)
	}

	contentType := aws.ToString(out.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return &Object{
		Body:        out.Body,
		ContentType: contentType,
		Size:        aws.ToInt64(out.ContentLength),
	}, nil
}
