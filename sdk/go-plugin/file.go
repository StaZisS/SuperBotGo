//go:build wasip1

package wasmplugin

import (
	"bytes"
	"fmt"
	"time"
)

//go:wasmimport env file_meta
func _file_meta(offset, length uint32) uint64

//go:wasmimport env file_read
func _file_read(offset, length uint32) uint64

//go:wasmimport env file_read_into
func _file_read_into(offset, length uint32) uint64

//go:wasmimport env file_url
func _file_url(offset, length uint32) uint64

//go:wasmimport env file_store
func _file_store(offset, length uint32) uint64

type fileMetaRequest struct {
	FileID string `msgpack:"file_id"`
}

type fileMetaResponse struct {
	ID       string `msgpack:"id"`
	Name     string `msgpack:"name"`
	MIMEType string `msgpack:"mime_type"`
	Size     int64  `msgpack:"size"`
	FileType string `msgpack:"file_type"`
	Error    string `msgpack:"error,omitempty"`
}

type fileReadRequest struct {
	FileID string `msgpack:"file_id"`
	Offset int64  `msgpack:"offset"`
	Length int64  `msgpack:"length"`
}

type fileReadResponse struct {
	Data      []byte `msgpack:"data"`
	BytesRead int64  `msgpack:"bytes_read"`
	EOF       bool   `msgpack:"eof"`
	Error     string `msgpack:"error,omitempty"`
}

type fileReadIntoRequest struct {
	FileID  string `msgpack:"file_id"`
	Offset  int64  `msgpack:"offset"`
	DataPtr uint32 `msgpack:"data_ptr"`
	DataLen uint32 `msgpack:"data_len"`
}

type fileReadIntoResponse struct {
	BytesRead int64  `msgpack:"bytes_read"`
	EOF       bool   `msgpack:"eof"`
	Error     string `msgpack:"error,omitempty"`
}

type fileURLRequest struct {
	FileID        string `msgpack:"file_id"`
	ExpirySeconds int    `msgpack:"expiry_seconds,omitempty"`
}

type fileURLResponse struct {
	URL   string `msgpack:"url"`
	Error string `msgpack:"error,omitempty"`
}

type fileStoreRequest struct {
	Name       string `msgpack:"name"`
	MIMEType   string `msgpack:"mime_type"`
	FileType   string `msgpack:"file_type"`
	Data       []byte `msgpack:"data"`
	TTLSeconds int    `msgpack:"ttl_seconds,omitempty"`
}

type fileStoreResponse struct {
	ID       string `msgpack:"id,omitempty"`
	Name     string `msgpack:"name,omitempty"`
	MIMEType string `msgpack:"mime_type,omitempty"`
	Size     int64  `msgpack:"size,omitempty"`
	FileType string `msgpack:"file_type,omitempty"`
	Error    string `msgpack:"error,omitempty"`
}

const maxFileReadChunkSize = 1 << 20 // 1 MB

// FileMeta returns metadata for a file by ID.
func (ctx *EventContext) FileMeta(fileID string) (*FileRef, error) {
	var resp fileMetaResponse
	if err := callHostWithResult(_file_meta, fileMetaRequest{FileID: fileID}, &resp); err != nil {
		return nil, fmt.Errorf("file_meta: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("file_meta: %s", resp.Error)
	}
	return &FileRef{
		ID:       resp.ID,
		Name:     resp.Name,
		MIMEType: resp.MIMEType,
		Size:     resp.Size,
		FileType: resp.FileType,
	}, nil
}

// FileRead reads up to maxBytes from a file starting at offset.
// Returns the data, whether EOF was reached, and any error.
// If maxBytes is 0, reads up to 1MB.
func (ctx *EventContext) FileRead(fileID string, offset int64, maxBytes int64) ([]byte, bool, error) {
	readLen := maxBytes
	if readLen <= 0 || readLen > maxFileReadChunkSize {
		readLen = maxFileReadChunkSize
	}
	buf := make([]byte, readLen)
	dataPtr, _ := bytesToPtr(buf)

	var resp fileReadIntoResponse
	if err := callHostWithResult(_file_read_into, fileReadIntoRequest{
		FileID:  fileID,
		Offset:  offset,
		DataPtr: dataPtr,
		DataLen: uint32(len(buf)),
	}, &resp); err != nil {
		return nil, false, fmt.Errorf("file_read_into: %w", err)
	}
	if resp.Error != "" {
		return nil, false, fmt.Errorf("file_read_into: %s", resp.Error)
	}
	if resp.BytesRead < 0 || resp.BytesRead > int64(len(buf)) {
		return nil, false, fmt.Errorf("file_read_into: invalid bytes_read %d", resp.BytesRead)
	}
	return buf[:resp.BytesRead], resp.EOF, nil
}

// FileReadAll reads the entire file content into memory.
// Only suitable for small files (< a few MB).
func (ctx *EventContext) FileReadAll(fileID string) ([]byte, error) {
	var buf bytes.Buffer
	var offset int64
	for {
		data, eof, err := ctx.FileRead(fileID, offset, 0)
		if err != nil {
			return nil, err
		}
		buf.Write(data)
		if eof {
			break
		}
		offset += int64(len(data))
	}
	return buf.Bytes(), nil
}

// FileURL returns a temporary download URL for the file.
// Returns "" if the backend does not support direct URLs.
func (ctx *EventContext) FileURL(fileID string) (string, error) {
	var resp fileURLResponse
	if err := callHostWithResult(_file_url, fileURLRequest{FileID: fileID}, &resp); err != nil {
		return "", fmt.Errorf("file_url: %w", err)
	}
	if resp.Error != "" {
		return "", fmt.Errorf("file_url: %s", resp.Error)
	}
	return resp.URL, nil
}

// FileStore stores a new file and returns a reference.
func (ctx *EventContext) FileStore(name, mimeType, fileType string, data []byte) (*FileRef, error) {
	return ctx.fileStoreInternal(name, mimeType, fileType, data, 0)
}

// FileStoreWithTTL stores a new file with a time-to-live.
func (ctx *EventContext) FileStoreWithTTL(name, mimeType, fileType string, data []byte, ttl time.Duration) (*FileRef, error) {
	return ctx.fileStoreInternal(name, mimeType, fileType, data, int(ttl.Seconds()))
}

func (ctx *EventContext) fileStoreInternal(name, mimeType, fileType string, data []byte, ttlSeconds int) (*FileRef, error) {
	var resp fileStoreResponse
	if err := callHostWithResult(_file_store, fileStoreRequest{
		Name:       name,
		MIMEType:   mimeType,
		FileType:   fileType,
		Data:       data,
		TTLSeconds: ttlSeconds,
	}, &resp); err != nil {
		return nil, fmt.Errorf("file_store: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("file_store: %s", resp.Error)
	}
	return &FileRef{
		ID:       resp.ID,
		Name:     resp.Name,
		MIMEType: resp.MIMEType,
		Size:     resp.Size,
		FileType: resp.FileType,
	}, nil
}
