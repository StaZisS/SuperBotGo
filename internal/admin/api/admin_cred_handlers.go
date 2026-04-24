package api

import (
	"net/http"
	"strconv"
)

// AdminCredHandler manages admin credential CRUD operations.
type AdminCredHandler struct {
	store  *PgAdminCredStore
	mailer AdminCredentialMailer
}

func NewAdminCredHandler(store *PgAdminCredStore, mailer AdminCredentialMailer) *AdminCredHandler {
	return &AdminCredHandler{store: store, mailer: mailer}
}

func (h *AdminCredHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/admins", h.handleList)
	mux.HandleFunc("GET /api/admin/admins/{userId}", h.handleGet)
	mux.HandleFunc("POST /api/admin/admins", h.handleCreate)
	mux.HandleFunc("PUT /api/admin/admins/{userId}/password", h.handleUpdatePassword)
	mux.HandleFunc("PUT /api/admin/admins/{userId}/email", h.handleUpdateEmail)
	mux.HandleFunc("DELETE /api/admin/admins/{userId}", h.handleDelete)
}

func (h *AdminCredHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.PathValue("userId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	cred, err := h.store.GetByUserID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get admin credentials")
		return
	}
	if cred == nil {
		writeJSON(w, http.StatusOK, map[string]any{"has_access": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"has_access": true, "credential": cred})
}

func (h *AdminCredHandler) handleList(w http.ResponseWriter, r *http.Request) {
	creds, err := h.store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list admin credentials")
		return
	}
	if creds == nil {
		creds = []AdminCredential{}
	}
	writeJSON(w, http.StatusOK, creds)
}

func (h *AdminCredHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GlobalUserID int64  `json:"global_user_id"`
		Email        string `json:"email"`
		Password     string `json:"password"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.GlobalUserID == 0 || req.Email == "" {
		writeError(w, http.StatusBadRequest, "global_user_id and email are required")
		return
	}
	generatedPassword := false
	if req.Password == "" {
		if h.mailer == nil {
			writeError(w, http.StatusBadRequest, "password is required when SMTP is not configured")
			return
		}
		password, err := generateTemporaryPassword()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate temporary password")
			return
		}
		req.Password = password
		generatedPassword = true
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	cred, err := h.store.Create(r.Context(), req.GlobalUserID, req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to create admin credentials: "+err.Error())
		return
	}
	if generatedPassword {
		if err := h.mailer.SendAdminCredentials(r.Context(), req.Email, req.Password); err != nil {
			_ = h.store.Delete(r.Context(), req.GlobalUserID)
			writeError(w, http.StatusBadGateway, "failed to send admin credentials email")
			return
		}
	}
	writeJSON(w, http.StatusCreated, cred)
}

func (h *AdminCredHandler) handleUpdatePassword(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.PathValue("userId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	if err := h.store.UpdatePassword(r.Context(), userID, req.Password); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *AdminCredHandler) handleUpdateEmail(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.PathValue("userId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	if err := h.store.UpdateEmail(r.Context(), userID, req.Email); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *AdminCredHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.PathValue("userId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	if err := h.store.Delete(r.Context(), userID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
