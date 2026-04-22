package hostapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"

	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/tetratelabs/wazero/api"
	"github.com/vmihailenco/msgpack/v5"
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

func readFileChunk(ctx context.Context, store filestore.FileStore, fileID string, offset, length int64) (fileReadResponse, error) {
	if offset < 0 {
		return fileReadResponse{}, fmt.Errorf("negative file offset: %d", offset)
	}

	readLen := length
	if readLen <= 0 || readLen > wasmrt.MaxFileChunkSize {
		readLen = wasmrt.MaxFileChunkSize
	}

	if rangeStore, ok := store.(filestore.RangeReader); ok {
		return readFileChunkFromReader(func() (io.ReadCloser, error) {
			rc, _, err := rangeStore.GetRange(ctx, fileID, offset, readLen)
			return rc, err
		}, readLen)
	}

	return readFileChunkFromReader(func() (io.ReadCloser, error) {
		rc, _, err := store.Get(ctx, fileID)
		if err != nil {
			return nil, err
		}
		if offset == 0 {
			return rc, nil
		}
		if _, err := io.CopyN(io.Discard, rc, offset); err != nil {
			rc.Close()
			if err == io.EOF {
				return io.NopCloser(bytes.NewReader(nil)), nil
			}
			return nil, err
		}
		return rc, nil
	}, readLen)
}

func readFileChunkInto(ctx context.Context, store filestore.FileStore, fileID string, offset int64, dst []byte) (int, bool, error) {
	if offset < 0 {
		return 0, false, fmt.Errorf("negative file offset: %d", offset)
	}
	if len(dst) == 0 {
		return 0, false, fmt.Errorf("empty file read buffer")
	}
	if len(dst) > int(wasmrt.MaxFileChunkSize) {
		dst = dst[:wasmrt.MaxFileChunkSize]
	}

	if rangeStore, ok := store.(filestore.RangeReader); ok {
		return readFileChunkIntoFromReader(func() (io.ReadCloser, error) {
			rc, _, err := rangeStore.GetRange(ctx, fileID, offset, int64(len(dst)))
			return rc, err
		}, dst)
	}

	return readFileChunkIntoFromReader(func() (io.ReadCloser, error) {
		rc, _, err := store.Get(ctx, fileID)
		if err != nil {
			return nil, err
		}
		if offset == 0 {
			return rc, nil
		}
		if _, err := io.CopyN(io.Discard, rc, offset); err != nil {
			rc.Close()
			if err == io.EOF {
				return io.NopCloser(bytes.NewReader(nil)), nil
			}
			return nil, err
		}
		return rc, nil
	}, dst)
}

func readFileChunkIntoFromReader(open func() (io.ReadCloser, error), dst []byte) (int, bool, error) {
	rc, err := open()
	if err != nil {
		return 0, false, err
	}
	defer rc.Close()

	n, readErr := io.ReadFull(rc, dst)
	eof := readErr == io.EOF || readErr == io.ErrUnexpectedEOF
	if readErr != nil && !eof {
		return 0, false, readErr
	}
	return n, eof, nil
}

func readFileChunkFromReader(open func() (io.ReadCloser, error), readLen int64) (fileReadResponse, error) {
	rc, err := open()
	if err != nil {
		return fileReadResponse{}, err
	}
	defer rc.Close()

	buf := make([]byte, readLen)
	n, readErr := io.ReadFull(rc, buf)
	eof := readErr == io.EOF || readErr == io.ErrUnexpectedEOF
	if readErr != nil && !eof {
		return fileReadResponse{}, readErr
	}

	return fileReadResponse{
		Data:      buf[:n],
		BytesRead: int64(n),
		EOF:       eof,
	}, nil
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

		resp, err := readFileChunk(ctx, h.deps.FileStore, req.FileID, req.Offset, req.Length)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, fileReadResponse{Error: err.Error()})
			return
		}
		writeResult(ctx, mod, stack, resp)
	}
}

func (h *HostAPI) fileReadIntoFunc() api.GoModuleFunc {
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

		var req fileReadIntoRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.FileStore == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("FileStore"))
			return
		}
		if req.DataLen == 0 {
			returnError(ctx, mod, stack, fmt.Errorf("file_read_into: data_len must be > 0"))
			return
		}

		readLen := req.DataLen
		if readLen > uint32(wasmrt.MaxFileChunkSize) {
			readLen = uint32(wasmrt.MaxFileChunkSize)
		}

		mem := mod.Memory()
		if mem == nil {
			returnError(ctx, mod, stack, fmt.Errorf("module has no memory"))
			return
		}
		dst, ok := mem.Read(req.DataPtr, readLen)
		if !ok {
			returnError(ctx, mod, stack, fmt.Errorf("memory read out of bounds: offset=%d, length=%d", req.DataPtr, readLen))
			return
		}

		bytesRead, eof, err := readFileChunkInto(ctx, h.deps.FileStore, req.FileID, req.Offset, dst)
		if err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, fileReadIntoResponse{Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, fileReadIntoResponse{
			BytesRead: int64(bytesRead),
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

		if int64(len(req.Data)) > h.maxFileStoreSize {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, fileStoreResponse{
				Error: fmt.Sprintf("file too large: %d bytes (max %d)", len(req.Data), h.maxFileStoreSize),
			})
			return
		}

		meta := filestore.FileMeta{
			Name:     req.Name,
			MIMEType: req.MIMEType,
			Size:     int64(len(req.Data)),
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
