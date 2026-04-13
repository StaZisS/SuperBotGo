package channel

import (
	"context"
	"fmt"
	"io"

	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"
)

type OpenedFile struct {
	Reader io.ReadCloser
	Ref    model.FileRef
}

// OpenFileRef loads file content and merges missing metadata from the file
// store into the provided lightweight FileRef.
func OpenFileRef(ctx context.Context, store filestore.FileStore, ref model.FileRef) (*OpenedFile, error) {
	if store == nil {
		return nil, fmt.Errorf("channel: file store is nil")
	}

	reader, meta, err := store.Get(ctx, ref.ID)
	if err != nil {
		return nil, err
	}

	resolved := ref
	if meta != nil {
		if resolved.Name == "" {
			resolved.Name = meta.Name
		}
		if resolved.MIMEType == "" {
			resolved.MIMEType = meta.MIMEType
		}
		if resolved.Size == 0 {
			resolved.Size = meta.Size
		}
		if resolved.FileType == "" {
			resolved.FileType = meta.FileType
		}
	}
	if resolved.Name == "" {
		resolved.Name = ref.ID
	}

	return &OpenedFile{
		Reader: reader,
		Ref:    resolved,
	}, nil
}
