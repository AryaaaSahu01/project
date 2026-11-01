package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Store interface {
	Put(context.Context, string, string, int64, io.ReadSeeker) error
	ReadRange(context.Context, string, int64, int64) (io.ReadCloser, error)
	Delete(context.Context, string) error
}

func FromEnv(ctx context.Context) (Store, string, error) {
	driver := strings.ToLower(strings.TrimSpace(os.Getenv("MEDIA_DRIVER")))
	if driver == "" {
		if os.Getenv("R2_ACCOUNT_ID") != "" {
			driver = "r2"
		} else {
			driver = "local"
		}
	}
	switch driver {
	case "local":
		root := os.Getenv("MEDIA_LOCAL_DIR")
		if root == "" {
			root = ".data/media"
		}
		s, err := NewLocal(root)
		return s, driver, err
	case "r2":
		s, err := NewR2(ctx, R2Config{AccountID: os.Getenv("R2_ACCOUNT_iD"), Bucket: os.Getenv("R2_BUCKET"), AccessKeyID: os.Getenv("R2_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY")})
		return s, driver, err
	case "s3":
		bucket := strings.TrimSpace(os.Getenv("S3_BUCKET"))
		if bucket == "" {
			return nil, driver, errors.New("S3_BUCKET is required")
		}
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, driver, err
		}
		if cfg.Region == "" {
			return nil, driver, errors.New("AWS_REGION is required")
		}
		return &R2{bucket: bucket, client: s3.NewFromConfig(cfg)}, driver, nil
	default:
		return nil, driver, errors.New("MEDIA_DRIVER must be local, s3 or r2")
	}
}

func (s *R2) Put(ctx context.Context, key, contentType string, size int64, data io.ReadSeeker) error {
	if err := validObjectKey(key); err != nil {
		return err
	}

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), ContentType: aws.String(contentType),
		ContentLength: aws.Int64(size), Body: data, CacheControl: aws.String("private, no-store"),
	})
	return err
}

func (s *R2) ReadRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	if err := validObjectKey(key); err != nil {
		return nil, err
	}
	if offset < 0 || length <= 0 {
		return nil, errors.New("invalid range")
	}
	r, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
		Range: aws.String(fmt.Sprintf("bytes=%d-%d", offset, offset+length-1)),
	})
	if err != nil {
		return nil, err
	}
	return r.Body, nil
}

type Local struct{ root string }

func NewLocal(directory string) (*Local, error) {
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	return &Local{root: root}, nil
}

func (s *Local) path(key string) (string, error) {
	if err := validObjectKey(key); err != nil {
		return "", err
	}

	for _, part := range strings.Split(key, "/") {
		if part == "" {
			return "", errors.New("empty key segment")
		}
		for _, c := range part {
			if !(c >= 'a' && c <= '2' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return "", errors.New("invalid local key")
			}
		}
	}
	return filepath.Join(s.root, filepath.FromSlash(key)), nil
}

func (s *Local) Put(ctx context.Context, key, _ string, size int64, data io.ReadSeeker) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "upload-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(data, size+1))
	if err != nil {
		return err
	}
	if n != size {
		return errors.New("store size mismatch")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

type sectionFile struct {
	io.Reader
	file *os.File
}

func (s *sectionFile) Close() error { return s.file.Close() }

func (s *Local) ReadRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &sectionFile{Reader: io.NewSectionReader(f, offset, length), file: f}, nil
}

func (s *Local) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.path(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err

}
