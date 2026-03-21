package trigger

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"SuperBotGo/internal/model"
)

// HTTPTriggerHandler serves incoming HTTP requests and dispatches them to plugins.
// URL scheme: /api/triggers/http/{pluginID}/{triggerPath...}
type HTTPTriggerHandler struct {
	router   *Router
	registry *Registry
	basePath string // e.g. "/api/triggers/http/"
}

func NewHTTPTriggerHandler(router *Router, registry *Registry) *HTTPTriggerHandler {
	return &HTTPTriggerHandler{
		router:   router,
		registry: registry,
		basePath: "/api/triggers/http/",
	}
}

func (h *HTTPTriggerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse pluginID and path from URL.
	// URL: /api/triggers/http/{pluginID}/{path...}
	remainder := strings.TrimPrefix(r.URL.Path, h.basePath)
	remainder = strings.TrimPrefix(remainder, "/")
	if remainder == "" {
		http.Error(w, "missing plugin ID in URL", http.StatusBadRequest)
		return
	}

	parts := strings.SplitN(remainder, "/", 2)
	pluginID := parts[0]
	triggerPath := ""
	if len(parts) > 1 {
		triggerPath = parts[1]
	}

	// Lookup trigger in registry.
	triggerName, err := h.registry.LookupHTTP(pluginID, triggerPath, r.Method)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Check API key auth.
	apiKey := h.registry.GetAPIKey(pluginID)
	if apiKey != "" {
		provided := r.Header.Get("X-Trigger-Key")
		if provided != apiKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Read body.
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	// Build query map.
	query := make(map[string]string, len(r.URL.Query()))
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			query[k] = v[0]
		}
	}

	// Build headers map.
	headers := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	// Build event.
	triggerData := model.HTTPTriggerData{
		Method:     r.Method,
		Path:       "/" + triggerPath,
		Query:      query,
		Headers:    headers,
		Body:       string(body),
		RemoteAddr: r.RemoteAddr,
	}
	dataJSON, _ := json.Marshal(triggerData)

	event := model.Event{
		ID:          generateID(),
		TriggerType: model.TriggerHTTP,
		TriggerName: triggerName,
		PluginID:    pluginID,
		Timestamp:   time.Now().UnixMilli(),
		Data:        dataJSON,
	}

	// Dispatch.
	resp, err := h.router.RouteEvent(r.Context(), event)
	if err != nil {
		slog.Error("HTTP trigger dispatch failed", "plugin", pluginID, "trigger", triggerName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if resp.Error != "" {
		slog.Error("HTTP trigger plugin error", "plugin", pluginID, "trigger", triggerName, "error", resp.Error)
		http.Error(w, resp.Error, http.StatusInternalServerError)
		return
	}

	// Parse HTTP response data from plugin.
	var httpResp model.HTTPResponseData
	if len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, &httpResp); err != nil {
			slog.Error("HTTP trigger: failed to parse plugin HTTP response", "plugin", pluginID, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	// Write response.
	for k, v := range httpResp.Headers {
		w.Header().Set(k, v)
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	statusCode := httpResp.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	w.WriteHeader(statusCode)
	w.Write([]byte(httpResp.Body))
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
