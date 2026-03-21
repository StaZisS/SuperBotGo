package api

import (
	"net/http"

	"SuperBotGo/internal/authz"
)

type RuleSchemaHandler struct {
	builder *authz.RuleSchemaBuilder
}

func NewRuleSchemaHandler(builder *authz.RuleSchemaBuilder) *RuleSchemaHandler {
	return &RuleSchemaHandler{builder: builder}
}

func (h *RuleSchemaHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/rule-schema", h.handleGetSchema)
}

func (h *RuleSchemaHandler) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	schema := h.builder.Build(r.Context())
	writeJSON(w, http.StatusOK, schema)
}
