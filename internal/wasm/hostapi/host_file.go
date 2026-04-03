package hostapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"

	"github.com/tetratelabs/wazero/api"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	maxFileChunkSize = 1048576          // 1 MB per read chunk
	maxFileStoreSize = 50 * 1024 * 1024 // 50 MB max file size via host API
)

// --- file_meta ---

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

func (h *HostAPI) fileMetaFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "file"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req fileMetaRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.FileStore == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("FileStore"))
			return
		}

		meta, err := h.deps.FileStore.Meta(ctx, req.FileID)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, fileMetaResponse{Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, fileMetaResponse{
			ID:       meta.ID,
			Name:     meta.Name,
			MIMEType: meta.MIMEType,
			Size:     meta.Size,
			FileType: string(meta.FileType),
		})
	}
}

// --- file_read ---

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

func (h *HostAPI) fileReadFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "file"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req fileReadRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.FileStore == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("FileStore"))
			return
		}

		// Clamp length to max chunk size
		readLen := req.Length
		if readLen <= 0 || readLen > maxFileChunkSize {
			readLen = maxFileChunkSize
		}

		rc, _, err := h.deps.FileStore.Get(ctx, req.FileID)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, fileReadResponse{Error: err.Error()})
			return
		}
		defer rc.Close()

		// Skip to offset without loading entire file into memory.
		if req.Offset > 0 {
			if _, err := io.CopyN(io.Discard, rc, req.Offset); err != nil {
				if err == io.EOF {
					writeResult(ctx, mod, stack, fileReadResponse{
						Data: []byte{}, BytesRead: 0, EOF: true,
					})
					return
				}
				SetHostCallStatus(ctx, "error")
				writeResult(ctx, mod, stack, fileReadResponse{Error: err.Error()})
				return
			}
		}

		// Read only the requested chunk.
		buf := make([]byte, readLen)
		n, readErr := io.ReadFull(rc, buf)
		eof := readErr == io.EOF || readErr == io.ErrUnexpectedEOF
		if readErr != nil && !eof {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, fileReadResponse{Error: readErr.Error()})
			return
		}

		writeResult(ctx, mod, stack, fileReadResponse{
			Data:      buf[:n],
			BytesRead: int64(n),
			EOF:       eof,
		})
	}
}

// --- file_url ---

type fileURLRequest struct {
	FileID        string `msgpack:"file_id"`
	ExpirySeconds int    `msgpack:"expiry_seconds"`
}

type fileURLResponse struct {
	URL   string `msgpack:"url"`
	Error string `msgpack:"error,omitempty"`
}

func (h *HostAPI) fileURLFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "file"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req fileURLRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.FileStore == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("FileStore"))
			return
		}

		expiry := time.Duration(req.ExpirySeconds) * time.Second
		if req.ExpirySeconds <= 0 {
			expiry = 3600 * time.Second
		}

		url, err := h.deps.FileStore.URL(ctx, req.FileID, expiry)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, fileURLResponse{Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, fileURLResponse{URL: url})
	}
}

// --- file_store ---

type fileStoreRequest struct {
	Name       string `msgpack:"name"`
	MIMEType   string `msgpack:"mime_type"`
	FileType   string `msgpack:"file_type"`
	Data       []byte `msgpack:"data"`
	TTLSeconds int    `msgpack:"ttl_seconds"`
}

type fileStoreResponse struct {
	ID       string `msgpack:"id"`
	Name     string `msgpack:"name"`
	MIMEType string `msgpack:"mime_type"`
	Size     int64  `msgpack:"size"`
	FileType string `msgpack:"file_type"`
	Error    string `msgpack:"error,omitempty"`
}

func (h *HostAPI) fileStoreFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		if err := h.perms.CheckPermission(pluginID, "file"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req fileStoreRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.FileStore == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("FileStore"))
			return
		}

		if int64(len(req.Data)) > maxFileStoreSize {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, fileStoreResponse{
				Error: fmt.Sprintf("file too large: %d bytes (max %d)", len(req.Data), maxFileStoreSize),
			})
			return
		}

		meta := filestore.FileMeta{
			Name:     req.Name,
			MIMEType: req.MIMEType,
			FileType: model.FileType(req.FileType),
			PluginID: pluginID,
		}

		if req.TTLSeconds > 0 {
			exp := time.Now().Add(time.Duration(req.TTLSeconds) * time.Second)
			meta.ExpiresAt = &exp
		}

		ref, err := h.deps.FileStore.Store(ctx, meta, bytes.NewReader(req.Data))
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, fileStoreResponse{Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, fileStoreResponse{
			ID:       ref.ID,
			Name:     ref.Name,
			MIMEType: ref.MIMEType,
			Size:     ref.Size,
			FileType: string(ref.FileType),
		})
	}
}
