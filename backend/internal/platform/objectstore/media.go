package objectstore

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (s *R2) Copy(ctx context.Context, from, to string) error {
	if err := validObjectKey(from); err != nil {
		return err
	}
	if err := validObjectKey(to); err != nil {
		return err
	}
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		Key:        aws.String(to),
		CopySource: aws.String(s.bucket + "/" + from),
	})
	return err
}

func (s *R2) DetectContentType(ctx context.Context, key string) (string, error) {
	if err := validObjectKey(key); err != nil {
		return "", err
	}
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Range:  aws.String("bytes=0-511"),
	})
	if err != nil {
		return "", err
	}
	defer result.Body.Close()
	data, err := io.ReadAll(io.LimitReader(result.Body, 512))
	if err != nil {
		return "", fmt.Errorf("inspect R2 object: %w", err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty R2 object")
	}
	return http.DetectContentType(data), nil
}
