package filehttp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"
)

const (
	maxRequestBodySize     int64 = 1 << 20
	defaultDirectUploadTTL       = 15 * time.Minute
)

type Handler struct {
	store                 filestore.FileStore
	directStore           filestore.DirectUploadStore
	maxFileSize           int64
	uploadTTL             time.Duration
	authenticateUser      func(r *http.Request) (model.GlobalUserID, bool)
	authenticateUserToken func(ctx context.Context, rawToken string) (model.GlobalUserID, bool, error)
	pluginExists          func(pluginID string) bool
	pluginAllowsFiles     func(pluginID string) bool
}

func NewHandler(store filestore.FileStore, maxFileSize int64) *Handler {
	h := &Handler{
		store:       store,
		maxFileSize: maxFileSize,
		uploadTTL:   defaultDirectUploadTTL,
	}
	if directStore, ok := store.(filestore.DirectUploadStore); ok {
		h.directStore = directStore
	}
	return h
}

func (h *Handler) SetUploadTTL(ttl time.Duration) {
	if ttl <= 0 {
		h.uploadTTL = defaultDirectUploadTTL
		return
	}
	h.uploadTTL = ttl
}

func (h *Handler) SetUserAuthenticator(fn func(r *http.Request) (model.GlobalUserID, bool)) {
	h.authenticateUser = fn
}

func (h *Handler) SetUserTokenAuthenticator(fn func(ctx context.Context, rawToken string) (model.GlobalUserID, bool, error)) {
	h.authenticateUserToken = fn
}

func (h *Handler) SetPluginExists(fn func(pluginID string) bool) {
	h.pluginExists = fn
}

func (h *Handler) SetPluginAllowsFiles(fn func(pluginID string) bool) {
	h.pluginAllowsFiles = fn
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/files/init", h.handleInit)
	mux.HandleFunc("POST /api/files/{id}/complete", h.handleComplete)
	mux.HandleFunc("DELETE /api/files/{id}", h.handleDelete)
}

func (h *Handler) handleInit(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	if h.directStore == nil {
		writeError(w, http.StatusServiceUnavailable, "direct uploads are unavailable")
		return
	}

	var body struct {
		PluginID string `json:"plugin_id"`
		Name     string `json:"name"`
		MIMEType string `json:"mime_type"`
		Size     int64  `json:"size"`
		FileType string `json:"file_type"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}

	body.PluginID = strings.TrimSpace(body.PluginID)
	body.Name = strings.TrimSpace(body.Name)
	body.MIMEType = strings.TrimSpace(body.MIMEType)
	if body.PluginID == "" {
		writeError(w, http.StatusBadRequest, "plugin_id is required")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.MIMEType == "" {
		writeError(w, http.StatusBadRequest, "mime_type is required")
		return
	}
	if body.Size < 0 {
		writeError(w, http.StatusBadRequest, "size must be >= 0")
		return
	}
	if h.maxFileSize > 0 && body.Size > h.maxFileSize {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("file too large: %d bytes (max %d)", body.Size, h.maxFileSize))
		return
	}
	if h.pluginExists != nil && !h.pluginExists(body.PluginID) {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}
	if h.pluginAllowsFiles != nil && !h.pluginAllowsFiles(body.PluginID) {
		writeError(w, http.StatusBadRequest, "plugin does not have file access")
		return
	}

	meta := filestore.FileMeta{
		Name:          body.Name,
		MIMEType:      body.MIMEType,
		Size:          body.Size,
		FileType:      normalizeFileType(body.FileType),
		State:         filestore.FileStatePending,
		ScopePluginID: body.PluginID,
		OwnerKind:     filestore.FileOwnerUser,
		OwnerUserID:   userID,
		ExpectedSize:  body.Size,
	}

	upload, err := h.directStore.CreateDirectUpload(r.Context(), meta, h.uploadTTL)
	if err != nil {
		slog.Error("file upload init failed", "plugin_id", body.PluginID, "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to initialize upload")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"file_id":        upload.FileID,
		"upload_url":     upload.URL,
		"upload_method":  upload.Method,
		"upload_headers": upload.Headers,
		"expires_at":     upload.ExpiresAt,
	})
}

func (h *Handler) handleComplete(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	if h.directStore == nil {
		writeError(w, http.StatusServiceUnavailable, "direct uploads are unavailable")
		return
	}

	fileID := strings.TrimSpace(r.PathValue("id"))
	if fileID == "" {
		writeError(w, http.StatusBadRequest, "file ID is required")
		return
	}

	meta, err := h.store.Meta(r.Context(), fileID)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err := authorizePendingUpload(meta, userID); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	ref, err := h.directStore.CompleteDirectUpload(r.Context(), fileID)
	if err != nil {
		slog.Error("file upload complete failed", "file_id", fileID, "user_id", userID, "error", err)
		writeError(w, http.StatusBadRequest, "failed to complete upload")
		return
	}

	if h.maxFileSize > 0 && ref.Size > h.maxFileSize {
		_ = h.store.Delete(r.Context(), fileID)
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("file too large: %d bytes (max %d)", ref.Size, h.maxFileSize))
		return
	}
	if meta.ExpectedSize > 0 && ref.Size != meta.ExpectedSize {
		_ = h.store.Delete(r.Context(), fileID)
		writeError(w, http.StatusBadRequest, fmt.Sprintf("uploaded size mismatch: got %d bytes, want %d", ref.Size, meta.ExpectedSize))
		return
	}

	writeJSON(w, http.StatusOK, ref)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUser(w, r)
	if !ok {
		return
	}

	fileID := strings.TrimSpace(r.PathValue("id"))
	if fileID == "" {
		writeError(w, http.StatusBadRequest, "file ID is required")
		return
	}

	meta, err := h.store.Meta(r.Context(), fileID)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err := authorizeOwnedUpload(meta, userID); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if err := h.store.Delete(r.Context(), fileID); err != nil {
		slog.Error("file delete failed", "file_id", fileID, "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete file")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) requireUser(w http.ResponseWriter, r *http.Request) (model.GlobalUserID, bool) {
	if token, ok := bearerToken(r); ok {
		if h.authenticateUserToken == nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return 0, false
		}
		userID, found, err := h.authenticateUserToken(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return 0, false
		}
		if !found {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return 0, false
		}
		return userID, true
	}

	if h.authenticateUser != nil {
		if userID, ok := h.authenticateUser(r); ok {
			return userID, true
		}
	}

	writeError(w, http.StatusUnauthorized, "authentication required")
	return 0, false
}

func authorizePendingUpload(meta *filestore.FileMeta, userID model.GlobalUserID) error {
	if err := authorizeOwnedUpload(meta, userID); err != nil {
		return err
	}
	if meta.State == filestore.FileStateReady {
		return nil
	}
	if meta.State != "" && meta.State != filestore.FileStatePending {
		return fmt.Errorf("file is not pending upload")
	}
	if meta.ExpiresAt != nil && meta.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("upload expired")
	}
	return nil
}

func authorizeOwnedUpload(meta *filestore.FileMeta, userID model.GlobalUserID) error {
	if meta == nil {
		return fmt.Errorf("file not found")
	}
	if meta.OwnerKind != filestore.FileOwnerUser || meta.OwnerUserID != userID {
		return fmt.Errorf("forbidden")
	}
	return nil
}

func normalizeFileType(raw string) model.FileType {
	switch model.FileType(strings.TrimSpace(strings.ToLower(raw))) {
	case model.FileTypePhoto,
		model.FileTypeDocument,
		model.FileTypeAudio,
		model.FileTypeVideo,
		model.FileTypeVoice,
		model.FileTypeSticker:
		return model.FileType(strings.TrimSpace(strings.ToLower(raw)))
	default:
		return model.FileTypeDocument
	}
}

func bearerToken(r *http.Request) (string, bool) {
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
	if token == "" {
		return "", false
	}
	return token, true
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("file api: failed to encode JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	slog.Warn("file api: API error", "status", status, "message", message)
	writeJSON(w, status, map[string]string{"error": message})
}
