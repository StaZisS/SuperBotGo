package api

import (
	"context"
	"io"
)

// BlobStore abstracts binary object storage for .wasm files.
type BlobStore interface {
	Put(ctx context.Context, key string, data io.Reader, size int64) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}
