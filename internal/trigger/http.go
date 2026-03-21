package trigger

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/model"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

type HTTPTriggerHandler struct {
	router   *Router
	registry *Registry
	basePath string
	metrics  *metrics.Metrics
}

func NewHTTPTriggerHandler(router *Router, registry *Registry) *HTTPTriggerHandler {
	return &HTTPTriggerHandler{
		router:   router,
		registry: registry,
		basePath: "/api/triggers/http/",
	}
}

func (h *HTTPTriggerHandler) SetMetrics(m *metrics.Metrics) {
	h.metrics = m
}

func (h *HTTPTriggerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
	var pluginID string
	defer func() {
		duration := time.Since(start)
		if h.metrics != nil && pluginID != "" {
			h.metrics.HTTPTriggerDuration.WithLabelValues(pluginID, r.Method).Observe(duration.Seconds())
			h.metrics.HTTPTriggerTotal.WithLabelValues(pluginID, r.Method, strconv.Itoa(rec.statusCode)).Inc()
		}
		slog.Info("http trigger",
			"plugin_id", pluginID,
			"method", r.Method,
			"path", r.URL.Path,
			"status_code", rec.statusCode,
			"duration_ms", duration.Milliseconds(),
		)
	}()

	remainder := strings.TrimPrefix(r.URL.Path, h.basePath)
	remainder = strings.TrimPrefix(remainder, "/")
	if remainder == "" {
		http.Error(rec, "missing plugin ID in URL", http.StatusBadRequest)
		return
	}

	parts := strings.SplitN(remainder, "/", 2)
	pluginID = parts[0]
	triggerPath := ""
	if len(parts) > 1 {
		triggerPath = parts[1]
	}

	triggerName, err := h.registry.LookupHTTP(pluginID, triggerPath, r.Method)
	if err != nil {
		http.Error(rec, err.Error(), http.StatusNotFound)
		return
	}

	apiKey := h.registry.GetAPIKey(pluginID)
	if apiKey != "" {
		provided := r.Header.Get("X-Trigger-Key")
		if provided != apiKey {
			http.Error(rec, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		http.Error(rec, "failed to read request body", http.StatusBadRequest)
		return
	}

	query := make(map[string]string, len(r.URL.Query()))
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			query[k] = v[0]
		}
	}

	headers := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

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

	resp, err := h.router.RouteEvent(r.Context(), event)
	if err != nil {
		slog.Error("HTTP trigger dispatch failed", "plugin", pluginID, "trigger", triggerName, "error", err)
		http.Error(rec, "internal error", http.StatusInternalServerError)
		return
	}

	if resp.Error != "" {
		slog.Error("HTTP trigger plugin error", "plugin", pluginID, "trigger", triggerName, "error", resp.Error)
		http.Error(rec, resp.Error, http.StatusInternalServerError)
		return
	}

	var httpResp model.HTTPResponseData
	if len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, &httpResp); err != nil {
			slog.Error("HTTP trigger: failed to parse plugin HTTP response", "plugin", pluginID, "error", err)
			http.Error(rec, "internal error", http.StatusInternalServerError)
			return
		}
	}

	for k, v := range httpResp.Headers {
		rec.Header().Set(k, v)
	}
	if rec.Header().Get("Content-Type") == "" {
		rec.Header().Set("Content-Type", "application/json")
	}
	statusCode := httpResp.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	rec.WriteHeader(statusCode)
	rec.Write([]byte(httpResp.Body))
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
