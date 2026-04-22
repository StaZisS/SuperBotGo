package filestore

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"SuperBotGo/internal/model"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3StoreConfig holds configuration for the S3-backed FileStore.
type S3StoreConfig struct {
	Bucket    string
	Region    string
	Endpoint  string
	AccessKey string
	SecretKey string
	Prefix    string // e.g. "files/"
}

// S3Store implements FileStore using S3-compatible object storage.
// Data is stored as <prefix><id>.data, metadata as <prefix><id>.meta.json.
type S3Store struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	prefix    string
}

// NewS3Store creates an S3-backed FileStore.
func NewS3Store(ctx context.Context, cfg S3StoreConfig) (*S3Store, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("filestore s3: bucket name is required")
	}

	var opts []func(*awsconfig.LoadOptions) error
	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("filestore s3: load aws config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
			o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	return &S3Store{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    cfg.Bucket,
		prefix:    cfg.Prefix,
	}, nil
}

func (s *S3Store) dataKey(id string) string { return s.prefix + id + ".data" }
func (s *S3Store) metaKey(id string) string { return s.prefix + id + ".meta.json" }

func (s *S3Store) Store(ctx context.Context, meta FileMeta, data io.Reader) (model.FileRef, error) {
	if meta.ID == "" {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		meta.ID = hex.EncodeToString(b)
	}
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now()
	}

	// Stream data upload and count bytes as they pass through.
	countingBody := &countingReader{reader: data}
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.dataKey(meta.ID)),
		Body:        countingBody,
		ContentType: aws.String(meta.MIMEType),
	}
	if meta.Size > 0 {
		putInput.ContentLength = aws.Int64(meta.Size)
	}

	_, err := s.client.PutObject(ctx, putInput)
	if err != nil {
		return model.FileRef{}, fmt.Errorf("filestore s3: put data %q: %w", meta.ID, err)
	}
	meta.Size = countingBody.n

	// Upload metadata.
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return model.FileRef{}, fmt.Errorf("filestore s3: marshal meta %q: %w", meta.ID, err)
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s.metaKey(meta.ID)),
		Body:          bytes.NewReader(metaBytes),
		ContentLength: aws.Int64(int64(len(metaBytes))),
		ContentType:   aws.String("application/json"),
	})
	if err != nil {
		// Cleanup data on meta failure.
		_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(s.dataKey(meta.ID)),
		})
		return model.FileRef{}, fmt.Errorf("filestore s3: put meta %q: %w", meta.ID, err)
	}

	return meta.Ref(), nil
}

func (s *S3Store) Get(ctx context.Context, id string) (io.ReadCloser, *FileMeta, error) {
	meta, err := s.Meta(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.dataKey(id)),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("filestore s3: get data %q: %w", id, err)
	}

	return out.Body, meta, nil
}

func (s *S3Store) GetRange(ctx context.Context, id string, offset, length int64) (io.ReadCloser, *FileMeta, error) {
	if offset < 0 {
		return nil, nil, fmt.Errorf("filestore s3: negative offset %d", offset)
	}

	meta, err := s.Meta(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if meta.Size <= 0 || offset >= meta.Size {
		return io.NopCloser(bytes.NewReader(nil)), meta, nil
	}

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.dataKey(id)),
	}
	switch {
	case length > 0:
		end := meta.Size - 1
		if remaining := meta.Size - offset; remaining > 0 && length < remaining {
			end = offset + length - 1
		}
		input.Range = aws.String(fmt.Sprintf("bytes=%d-%d", offset, end))
	case offset > 0:
		input.Range = aws.String(fmt.Sprintf("bytes=%d-", offset))
	}

	out, err := s.client.GetObject(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("filestore s3: get data range %q: %w", id, err)
	}

	return out.Body, meta, nil
}

func (s *S3Store) Meta(ctx context.Context, id string) (*FileMeta, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.metaKey(id)),
	})
	if err != nil {
		return nil, fmt.Errorf("filestore s3: get meta %q: %w", id, err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("filestore s3: read meta %q: %w", id, err)
	}

	var meta FileMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("filestore s3: unmarshal meta %q: %w", id, err)
	}
	return &meta, nil
}

func (s *S3Store) Delete(ctx context.Context, id string) error {
	_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.dataKey(id)),
	})
	_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.metaKey(id)),
	})
	return nil
}

// URL returns a presigned GET URL for downloading the file directly from S3.
func (s *S3Store) URL(ctx context.Context, id string, expiry time.Duration) (string, error) {
	if expiry <= 0 {
		expiry = 1 * time.Hour
	}
	out, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.dataKey(id)),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("filestore s3: presign %q: %w", id, err)
	}
	return out.URL, nil
}

// Cleanup lists all .meta.json objects, checks ExpiresAt, and deletes expired files.
func (s *S3Store) Cleanup(ctx context.Context) (int, error) {
	now := time.Now()
	removed := 0

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(s.prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return removed, fmt.Errorf("filestore s3: list objects: %w", err)
		}

		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if len(key) < 10 || key[len(key)-10:] != ".meta.json" {
				continue
			}

			// Read meta to check expiry.
			out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(s.bucket),
				Key:    obj.Key,
			})
			if err != nil {
				continue
			}

			data, err := io.ReadAll(out.Body)
			out.Body.Close()
			if err != nil {
				continue
			}

			var meta FileMeta
			if json.Unmarshal(data, &meta) != nil || meta.ID == "" {
				continue
			}

			if meta.ExpiresAt != nil && meta.ExpiresAt.Before(now) {
				_ = s.Delete(ctx, meta.ID)
				removed++
			}
		}
	}

	return removed, nil
}

func (s *S3Store) exists(ctx context.Context, key string) bool {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false
		}
		var noKey *types.NoSuchKey
		if errors.As(err, &noKey) {
			return false
		}
	}
	return err == nil
}

var _ FileStore = (*S3Store)(nil)
var _ RangeReader = (*S3Store)(nil)

type countingReader struct {
	reader io.Reader
	n      int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.n += int64(n)
	return n, err
}
