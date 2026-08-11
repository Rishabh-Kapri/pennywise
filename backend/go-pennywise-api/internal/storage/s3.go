package storage

import (
	"context"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Config points the store at any S3-compatible bucket. On Railway these come
// from the bucket's Credentials tab, exposed as the standard AWS_* variables.
type S3Config struct {
	Endpoint     string // e.g. https://storage.railway.app
	Bucket       string
	AccessKey    string
	SecretKey    string
	Region       string // Railway uses "auto"
	UsePathStyle bool   // Railway serves virtual-hosted style, so false
}

func (c S3Config) enabled() bool {
	return c.Bucket != "" && c.AccessKey != "" && c.SecretKey != ""
}

type s3Store struct {
	client *s3.Client
	bucket string
}

// NewS3Store builds a Store backed by an S3-compatible bucket.
//
// Object keys are the same relative paths the local store hands out, so
// storage_path rows written by either implementation stay meaningful and a
// switch does not require rewriting the table.
func NewS3Store(ctx context.Context, cfg S3Config) (Store, error) {
	if !cfg.enabled() {
		return nil, fmt.Errorf("s3 storage requires bucket, access key and secret key")
	}

	region := cfg.Region
	if region == "" {
		region = "auto"
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("error loading aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
	})

	return &s3Store{client: client, bucket: cfg.Bucket}, nil
}

// key normalises a relative path into an object key. Object stores have no
// directories, so the traversal guard the local store needs does not apply --
// but leading slashes and "." segments would still produce surprising keys.
func (s *s3Store) key(relPath string) (string, error) {
	cleaned := path.Clean("/" + strings.ReplaceAll(relPath, `\`, "/"))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || cleaned == "." {
		return "", fmt.Errorf("invalid storage path %q", relPath)
	}
	return cleaned, nil
}

func (s *s3Store) Save(dir string, fileName string, r io.Reader) (string, error) {
	relPath := path.Join(dir, fileName)
	objectKey, err := s.key(relPath)
	if err != nil {
		return "", err
	}

	// The SDK needs a seekable body to sign, and these are already bounded by
	// the upload limits (10MB per file, 40MB per scan), so buffering is safe.
	// Streaming would require a multipart upload for no benefit at this size.
	body, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("error reading upload body: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
		Body:   bytes.NewReader(body),
	})
	if err != nil {
		return "", fmt.Errorf("error uploading %q: %w", objectKey, err)
	}
	return relPath, nil
}

// tempFileReader is a ReadSeekCloser over a downloaded object that deletes
// itself when closed.
//
// http.ServeContent needs to seek (it serves range requests and sniffs content),
// but S3 GetObject returns a plain stream. Spooling to a temp file keeps range
// support without holding whole documents in memory.
type tempFileReader struct {
	*os.File
}

func (t *tempFileReader) Close() error {
	name := t.File.Name()
	err := t.File.Close()
	if rmErr := os.Remove(name); err == nil && rmErr != nil && !os.IsNotExist(rmErr) {
		err = rmErr
	}
	return err
}

func (s *s3Store) Open(relPath string) (io.ReadSeekCloser, error) {
	objectKey, err := s.key(relPath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		var noKey *types.NoSuchKey
		if errors.As(err, &noKey) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("error fetching %q: %w", objectKey, err)
	}
	defer out.Body.Close()

	tmp, err := os.CreateTemp("", "pennywise-doc-*")
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(tmp, out.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}
	return &tempFileReader{File: tmp}, nil
}

func (s *s3Store) Delete(relPath string) error {
	objectKey, err := s.key(relPath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		var noKey *types.NoSuchKey
		if errors.As(err, &noKey) {
			return nil
		}
		return fmt.Errorf("error deleting %q: %w", objectKey, err)
	}
	return nil
}
