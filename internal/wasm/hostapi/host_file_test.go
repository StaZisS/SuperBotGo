package hostapi

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

func TestReadFileChunkUsesRangeReader(t *testing.T) {
	store := &rangeCapableFileStore{
		basicFileStore: basicFileStore{data: []byte("abcdefghij")},
	}

	resp, err := readFileChunk(context.Background(), store, "file-1", 2, 4)
	if err != nil {
		t.Fatalf("readFileChunk() error = %v", err)
	}
	if store.getCalled {
		t.Fatal("expected range path, but Get was called")
	}
	if !store.getRangeCalled {
		t.Fatal("expected GetRange to be called")
	}
	if got := string(resp.Data); got != "cdef" {
		t.Fatalf("resp.Data = %q, want %q", got, "cdef")
	}
	if resp.EOF {
		t.Fatal("resp.EOF = true, want false")
	}
}

func TestReadFileChunkFallbackUsesSequentialRead(t *testing.T) {
	store := &basicFileStore{data: []byte("abcdefghij")}

	resp, err := readFileChunk(context.Background(), store, "file-1", 8, 4)
	if err != nil {
		t.Fatalf("readFileChunk() error = %v", err)
	}
	if !store.getCalled {
		t.Fatal("expected Get to be called")
	}
	if got := string(resp.Data); got != "ij" {
		t.Fatalf("resp.Data = %q, want %q", got, "ij")
	}
	if !resp.EOF {
		t.Fatal("resp.EOF = false, want true")
	}
}

func TestReadFileChunkRejectsNegativeOffset(t *testing.T) {
	store := &basicFileStore{data: []byte("abc")}

	_, err := readFileChunk(context.Background(), store, "file-1", -1, 1)
	if err == nil {
		t.Fatal("expected error for negative offset")
	}
}

func TestReadFileChunkIntoUsesRangeReader(t *testing.T) {
	store := &rangeCapableFileStore{
		basicFileStore: basicFileStore{data: []byte("abcdefghij")},
	}
	buf := make([]byte, 4)

	n, eof, err := readFileChunkInto(context.Background(), store, "file-1", 2, buf)
	if err != nil {
		t.Fatalf("readFileChunkInto() error = %v", err)
	}
	if store.getCalled {
		t.Fatal("expected range path, but Get was called")
	}
	if !store.getRangeCalled {
		t.Fatal("expected GetRange to be called")
	}
	if eof {
		t.Fatal("eof = true, want false")
	}
	if got := string(buf[:n]); got != "cdef" {
		t.Fatalf("buf = %q, want %q", got, "cdef")
	}
}

func TestHostAPISetMaxFileStoreSize(t *testing.T) {
	h := NewHostAPI(Dependencies{})
	if h.maxFileStoreSize != wasmrt.MaxFileStoreSize {
		t.Fatalf("default maxFileStoreSize = %d, want %d", h.maxFileStoreSize, wasmrt.MaxFileStoreSize)
	}

	h.SetMaxFileStoreSize(1234)
	if h.maxFileStoreSize != 1234 {
		t.Fatalf("configured maxFileStoreSize = %d, want %d", h.maxFileStoreSize, 1234)
	}

	h.SetMaxFileStoreSize(0)
	if h.maxFileStoreSize != wasmrt.MaxFileStoreSize {
		t.Fatalf("reset maxFileStoreSize = %d, want %d", h.maxFileStoreSize, wasmrt.MaxFileStoreSize)
	}
}

type basicFileStore struct {
	data      []byte
	getCalled bool
}

func (s *basicFileStore) Store(context.Context, filestore.FileMeta, io.Reader) (model.FileRef, error) {
	return model.FileRef{}, nil
}

func (s *basicFileStore) Get(context.Context, string) (io.ReadCloser, *filestore.FileMeta, error) {
	s.getCalled = true
	return io.NopCloser(bytes.NewReader(s.data)), &filestore.FileMeta{Size: int64(len(s.data))}, nil
}

func (s *basicFileStore) Meta(context.Context, string) (*filestore.FileMeta, error) {
	return &filestore.FileMeta{Size: int64(len(s.data))}, nil
}

func (s *basicFileStore) Delete(context.Context, string) error {
	return nil
}

func (s *basicFileStore) URL(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

func (s *basicFileStore) Cleanup(context.Context) (int, error) {
	return 0, nil
}

type rangeCapableFileStore struct {
	basicFileStore
	getRangeCalled bool
}

func (s *rangeCapableFileStore) GetRange(_ context.Context, _ string, offset, length int64) (io.ReadCloser, *filestore.FileMeta, error) {
	s.getRangeCalled = true
	if offset < 0 {
		return nil, nil, io.ErrUnexpectedEOF
	}
	if len(s.data) == 0 || offset >= int64(len(s.data)) {
		return io.NopCloser(bytes.NewReader(nil)), &filestore.FileMeta{}, nil
	}
	start := int(offset)
	end := len(s.data)
	if length > 0 {
		end = start + int(length)
		if end > len(s.data) {
			end = len(s.data)
		}
	}
	return io.NopCloser(bytes.NewReader(s.data[start:end])), &filestore.FileMeta{Size: int64(len(s.data))}, nil
}
