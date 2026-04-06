package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("admin: failed to encode JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	slog.Warn("admin: API error", "status", status, "message", message)
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func validateBlobKey(key string) bool {
	if key == "" {
		return false
	}
	if strings.Contains(key, "..") ||
		strings.Contains(key, "/") ||
		strings.Contains(key, "\\") ||
		strings.ContainsAny(key, "\x00") {
		return false
	}
	return true
}

// readWasmFromForm parses a multipart form, reads the "wasm" file field,
// validates the .wasm extension, and returns the raw bytes.
// Returns nil, false if an error response was already written.
func readWasmFromForm(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return nil, false
	}

	file, header, err := r.FormFile("wasm")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'wasm' file in form")
		return nil, false
	}
	defer file.Close()

	if !strings.HasSuffix(header.Filename, ".wasm") {
		writeError(w, http.StatusBadRequest, "file must have .wasm extension")
		return nil, false
	}

	wasmBytes, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read uploaded file")
		return nil, false
	}

	return wasmBytes, true
}
