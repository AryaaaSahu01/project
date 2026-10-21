package objectstore

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type R2Config struct {
	AccountID       string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
}

type R2 struct {
	bucket    string
	client    *s3.Client
	presigner *s3.PresignClient
}

func NewR2(ctx context.Context, in R2Config) (*R2, error) {
	if strings.TrimSpace(in.AccountID) == "" || strings.TrimSpace(in.Bucket) == "" ||
		strings.TrimSpace(in.AccessKeyID) == "" || strings.TrimSpace(in.SecretAccessKey) == "" {
		return nil, errors.New("R2 account ID, bucket and S3 credentials are required")
	}
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			in.AccessKeyID, in.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("configure R2: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://" + in.AccountID + ".r2.cloudflarestorage.com")
		o.UsePathStyle = true
	})
	return &R2{
		bucket:    in.Bucket,
		client:    client,
		presigner: s3.NewPresignClient(client),
	}, nil
}

// CheckBucket performs an authenticated request without reading private objects.
func (s *R2) CheckBucket(ctx context.Context) error {
	_, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket), MaxKeys: aws.Int32(1),
	})
	return err
}

func (s *R2) PresignUpload(ctx context.Context, key, contentType string) (string, http.Header, error) {
	if err := validObjectKey(key); err != nil {
		return "", nil, err
	}
	if !strings.HasPrefix(contentType, "image/") &&
		!strings.HasPrefix(contentType, "audio/") &&
		!strings.HasPrefix(contentType, "video/") {
		return "", nil, errors.New("unsupported media content type")
	}
	result, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(10*time.Minute))
	if err != nil {
		return "", nil, err
	}
	return result.URL, result.SignedHeader, nil
}

// PresignDownload MUST be called only AFTER application authorization and
// after validating any sealed memory has reached its unlock time.
func (s *R2) PresignDownload(ctx context.Context, key string) (string, error) {
	if err := validObjectKey(key); err != nil {
		return "", err
	}
	result, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	}, s3.WithPresignExpires(time.Minute))
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

// Head allows the later finalize API to verify the actual uploaded object.
func (s *R2) Head(ctx context.Context, key string) (size int64, contentType string, err error) {
	if err := validObjectKey(key); err != nil {
		return 0, "", err
	}
	o, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	})
	if err != nil {
		return 0, "", err
	}
	return aws.ToInt64(o.ContentLength), aws.ToString(o.ContentType), nil
}

func (s *R2) Delete(ctx context.Context, key string) error {
	if err := validObjectKey(key); err != nil {
		return err
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	})
	return err
}

func validObjectKey(key string) error {
	if len(key) == 0 || len(key) > 512 ||
		strings.HasPrefix(key, "/") || strings.Contains(key, "..") ||
		strings.ContainsAny(key, "\\\r\n") {
		return errors.New("invalid object key")
	}
	return nil
}
