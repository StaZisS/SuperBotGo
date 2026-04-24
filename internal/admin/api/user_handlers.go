package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"SuperBotGo/internal/model"
)

type UserDetail struct {
	ID             int64             `json:"id"`
	PrimaryChannel model.ChannelType `json:"primary_channel"`
	TsuAccountsID  *string           `json:"tsu_accounts_id,omitempty"`
	Locale         string            `json:"locale"`
	Role           string            `json:"role"`
	PersonName     string            `json:"person_name,omitempty"`
	ProfileData    map[string]any    `json:"profile_data,omitempty"`
	Accounts       []AccountInfo     `json:"accounts"`
	CreatedAt      *time.Time        `json:"created_at,omitempty"`
}

type AccountInfo struct {
	ID            int64             `json:"id"`
	ChannelType   model.ChannelType `json:"channel_type"`
	ChannelUserID string            `json:"channel_user_id"`
	Username      string            `json:"username,omitempty"`
	LinkedAt      time.Time         `json:"linked_at"`
}

type AccountBrief struct {
	ChannelType model.ChannelType `json:"channel_type"`
	Username    string            `json:"username,omitempty"`
}

type UserListItem struct {
	ID         int64          `json:"id"`
	Locale     string         `json:"locale"`
	Role       string         `json:"role"`
	PersonName string         `json:"person_name,omitempty"`
	Accounts   []AccountBrief `json:"accounts"`
	CreatedAt  *time.Time     `json:"created_at,omitempty"`
}

type UpdateUserRequest struct {
	Locale      string         `json:"locale"`
	Role        string         `json:"role"`
	ProfileData map[string]any `json:"profile_data"`
}

type UserRoleEntry struct {
	ID        int64     `json:"id"`
	RoleName  string    `json:"role_name"`
	RoleType  string    `json:"role_type"`
	Scope     string    `json:"scope,omitempty"`
	GrantedAt time.Time `json:"granted_at"`
	GrantedBy *int64    `json:"granted_by,omitempty"`
}

type UserListOptions struct {
	Search  string
	Role    string
	Channel string
	Offset  int
	Limit   int
}

type UserStore interface {
	ListUsers(ctx context.Context, opts UserListOptions) ([]UserListItem, int, error)
	GetUser(ctx context.Context, id int64) (*UserDetail, error)
	UpdateUser(ctx context.Context, id int64, req UpdateUserRequest) error
	DeleteUser(ctx context.Context, id int64) error
	GetUserRoles(ctx context.Context, userID int64) ([]UserRoleEntry, error)
	RemoveUserRole(ctx context.Context, userID int64, roleName, roleType string) error
	UnlinkAccount(ctx context.Context, accountID int64) error
}

// SubjectInvalidator drops cached authorization state for a specific user.
// Wired up with an Authorizer so deleted users cannot continue to be
// authorized from stale cache entries.
type SubjectInvalidator interface {
	InvalidateUser(userID model.GlobalUserID)
}

type UserHandler struct {
	store       UserStore
	invalidator SubjectInvalidator
}

func NewUserHandler(store UserStore, invalidator ...SubjectInvalidator) *UserHandler {
	h := &UserHandler{store: store}
	if len(invalidator) > 0 {
		h.invalidator = invalidator[0]
	}
	return h
}

func (h *UserHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/users", h.handleListUsers)
	mux.HandleFunc("GET /api/admin/users/{id}", h.handleGetUser)
	mux.HandleFunc("PUT /api/admin/users/{id}", h.handleUpdateUser)
	mux.HandleFunc("DELETE /api/admin/users/{id}", h.handleDeleteUser)
	mux.HandleFunc("GET /api/admin/users/{id}/roles", h.handleGetUserRoles)
	mux.HandleFunc("DELETE /api/admin/users/{id}/roles", h.handleRemoveUserRole)
	mux.HandleFunc("DELETE /api/admin/users/{id}/accounts/{accountId}", h.handleUnlinkAccount)
}

func (h *UserHandler) handleListUsers(w http.ResponseWriter, r *http.Request) {
	opts := UserListOptions{
		Search:  r.URL.Query().Get("search"),
		Role:    r.URL.Query().Get("role"),
		Channel: r.URL.Query().Get("channel"),
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		opts.Offset, _ = strconv.Atoi(offset)
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		opts.Limit, _ = strconv.Atoi(limit)
	} else {
		opts.Limit = 50
	}

	users, total, err := h.store.ListUsers(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"users": users, "total": total})
}

func (h *UserHandler) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	user, err := h.store.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *UserHandler) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req UpdateUserRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	if err := h.store.UpdateUser(r.Context(), id, req); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *UserHandler) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	if err := h.store.DeleteUser(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	if h.invalidator != nil {
		h.invalidator.InvalidateUser(model.GlobalUserID(id))
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *UserHandler) handleGetUserRoles(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	roles, err := h.store.GetUserRoles(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusOK, []UserRoleEntry{})
		return
	}
	writeJSON(w, http.StatusOK, roles)
}

func (h *UserHandler) handleRemoveUserRole(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req struct {
		RoleName string `json:"role_name"`
		RoleType string `json:"role_type"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	if err := h.store.RemoveUserRole(r.Context(), id, req.RoleName, req.RoleType); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove role")
		return
	}
	if h.invalidator != nil {
		h.invalidator.InvalidateUser(model.GlobalUserID(id))
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *UserHandler) handleUnlinkAccount(w http.ResponseWriter, r *http.Request) {
	accountID, err := strconv.ParseInt(r.PathValue("accountId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account ID")
		return
	}

	if err := h.store.UnlinkAccount(r.Context(), accountID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlink account")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unlinked"})
}
