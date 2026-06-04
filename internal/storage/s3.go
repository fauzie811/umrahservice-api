package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"umrahservice-api/internal/config"
)

// Storage wraps the S3 client and mirrors Laravel's Storage::disk('s3')
// helpers used by the API (store + url).
type Storage struct {
	client *s3.Client
	bucket string
	url    string
}

// New builds an S3-backed Storage from config (path-style endpoint supported).
func New(cfg *config.Config) (*Storage, error) {
	awsCfg, err := awscfg.LoadDefaultConfig(context.Background(),
		awscfg.WithRegion(cfg.AWS.Region),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AWS.AccessKeyID, cfg.AWS.SecretAccessKey, "",
		)),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.AWS.Endpoint != "" {
			o.BaseEndpoint = awssdk.String(cfg.AWS.Endpoint)
		}
		o.UsePathStyle = cfg.AWS.UsePathStyle
	})

	return &Storage{
		client: client,
		bucket: cfg.AWS.Bucket,
		url:    cfg.AWS.URL,
	}, nil
}

// Store uploads content to {dir}/{random40}.{ext} and returns the stored key,
// mirroring Laravel's UploadedFile::store('dir', 's3') hashName behaviour.
func (s *Storage) Store(ctx context.Context, dir string, ext string, contentType string, content []byte) (string, error) {
	key := path.Join(dir, hashName(ext))
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      awssdk.String(s.bucket),
		Key:         awssdk.String(key),
		Body:        bytes.NewReader(content),
		ContentType: awssdk.String(contentType),
	})
	if err != nil {
		return "", err
	}
	return key, nil
}

// Delete removes an object by key (best-effort).
func (s *Storage) Delete(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: awssdk.String(s.bucket),
		Key:    awssdk.String(key),
	})
	return err
}

// URL mirrors Storage::disk('s3')->url($path): AWS_URL + path.
func (s *Storage) URL(key string) string {
	if key == "" {
		return ""
	}
	return strings.TrimRight(s.url, "/") + "/" + strings.TrimLeft(key, "/")
}

// hashName mirrors Laravel's File::hashName (40 random chars + extension).
func hashName(ext string) string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	name := hex.EncodeToString(b)
	if ext == "" {
		return name
	}
	return fmt.Sprintf("%s.%s", name, strings.TrimPrefix(ext, "."))
}
