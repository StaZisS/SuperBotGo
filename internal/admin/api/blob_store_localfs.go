package api

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalFSBlobStore implements BlobStore using the local filesystem.
type LocalFSBlobStore struct {
	baseDir string
}

// NewLocalFSBlobStore creates a BlobStore backed by the local filesystem.
// It ensures baseDir exists on creation.
func NewLocalFSBlobStore(baseDir string) (*LocalFSBlobStore, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create blob base dir %q: %w", baseDir, err)
	}
	return &LocalFSBlobStore{baseDir: baseDir}, nil
}

func (s *LocalFSBlobStore) path(key string) string {
	return filepath.Join(s.baseDir, key)
}

// Put writes data to a file identified by key.
func (s *LocalFSBlobStore) Put(_ context.Context, key string, data io.Reader, _ int64) error {
	p := s.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("create parent dir for %q: %w", key, err)
	}

	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("create blob file %q: %w", key, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, data); err != nil {
		_ = os.Remove(p)
		return fmt.Errorf("write blob %q: %w", key, err)
	}
	return nil
}

// Get returns a ReadCloser for the blob identified by key.
func (s *LocalFSBlobStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	f, err := os.Open(s.path(key))
	if err != nil {
		return nil, fmt.Errorf("open blob %q: %w", key, err)
	}
	return f, nil
}

// Delete removes the blob identified by key.
func (s *LocalFSBlobStore) Delete(_ context.Context, key string) error {
	err := os.Remove(s.path(key))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete blob %q: %w", key, err)
	}
	return nil
}

// Exists checks whether a blob with the given key exists.
func (s *LocalFSBlobStore) Exists(_ context.Context, key string) (bool, error) {
	_, err := os.Stat(s.path(key))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("stat blob %q: %w", key, err)
}

var _ BlobStore = (*LocalFSBlobStore)(nil)
