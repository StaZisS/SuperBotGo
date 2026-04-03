package filestore

import (
	"context"
	"io"
	"time"

	"SuperBotGo/internal/model"
)

// FileMeta holds the full metadata for a stored file.
type FileMeta struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	MIMEType  string         `json:"mime_type"`
	Size      int64          `json:"size"`
	FileType  model.FileType `json:"file_type"`
	PluginID  string         `json:"plugin_id,omitempty"` // who stored it ("" for incoming)
	CreatedAt time.Time      `json:"created_at"`
	ExpiresAt *time.Time     `json:"expires_at,omitempty"`
}

// Ref converts FileMeta to a lightweight FileRef suitable for passing to plugins.
func (m *FileMeta) Ref() model.FileRef {
	return model.FileRef{
		ID:       m.ID,
		Name:     m.Name,
		MIMEType: m.MIMEType,
		Size:     m.Size,
		FileType: m.FileType,
	}
}

// FileStore provides file storage with metadata, TTL and cleanup.
type FileStore interface {
	// Store saves file content and returns a reference.
	Store(ctx context.Context, meta FileMeta, data io.Reader) (model.FileRef, error)

	// Get retrieves file content and metadata by ID.
	Get(ctx context.Context, id string) (io.ReadCloser, *FileMeta, error)

	// Meta retrieves metadata only (no content).
	Meta(ctx context.Context, id string) (*FileMeta, error)

	// Delete removes a file.
	Delete(ctx context.Context, id string) error

	// URL returns a temporary direct download URL for the file.
	// Returns ("", nil) if the backend does not support direct URLs.
	URL(ctx context.Context, id string, expiry time.Duration) (string, error)

	// Cleanup removes expired files. Returns the number of files removed.
	Cleanup(ctx context.Context) (int, error)
}
