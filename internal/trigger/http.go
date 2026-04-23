package trigger

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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

type HTTPTriggerSetting struct {
	Enabled          bool
	AllowUserKeys    bool
	AllowServiceKeys bool
	PolicyExpression string
}

type ServiceKeyPrincipal struct {
	ID int64
}

type resolvedHTTPPrincipal struct {
	authData *model.HTTPAuthData
}

type HTTPTriggerHandler struct {
	router   *Router
	registry *Registry
	basePath string
	metrics  *metrics.Metrics

	loadSetting           func(ctx context.Context, pluginID, triggerName string) (HTTPTriggerSetting, bool, error)
	authenticateUser      func(r *http.Request) (model.GlobalUserID, bool)
	authenticateUserToken func(ctx context.Context, rawToken string) (model.GlobalUserID, bool, error)
	authenticateService   func(ctx context.Context, rawToken, pluginID, triggerName string) (ServiceKeyPrincipal, bool, error)
	evalPolicy            func(ctx context.Context, expression string, userID model.GlobalUserID) (bool, error)
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

func (h *HTTPTriggerHandler) SetSettingLoader(loader func(ctx context.Context, pluginID, triggerName string) (HTTPTriggerSetting, bool, error)) {
	h.loadSetting = loader
}

func (h *HTTPTriggerHandler) SetUserAuthenticator(fn func(r *http.Request) (model.GlobalUserID, bool)) {
	h.authenticateUser = fn
}

func (h *HTTPTriggerHandler) SetUserTokenAuthenticator(fn func(ctx context.Context, rawToken string) (model.GlobalUserID, bool, error)) {
	h.authenticateUserToken = fn
}

func (h *HTTPTriggerHandler) SetServiceAuthenticator(fn func(ctx context.Context, rawToken, pluginID, triggerName string) (ServiceKeyPrincipal, bool, error)) {
	h.authenticateService = fn
}

func (h *HTTPTriggerHandler) SetPolicyEvaluator(fn func(ctx context.Context, expression string, userID model.GlobalUserID) (bool, error)) {
	h.evalPolicy = fn
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

	setting, err := h.resolveSetting(r.Context(), pluginID, triggerName)
	if err != nil {
		slog.Error("HTTP trigger: failed to load access setting", "plugin", pluginID, "trigger", triggerName, "error", err)
		http.Error(rec, "internal error", http.StatusInternalServerError)
		return
	}
	if !setting.Enabled {
		http.Error(rec, "forbidden", http.StatusForbidden)
		return
	}

	principal, statusCode, err := h.resolvePrincipal(r, pluginID, triggerName, setting)
	if err != nil {
		if statusCode == 0 {
			statusCode = http.StatusInternalServerError
		}
		if redirectURL, ok := loginRedirectURL(r, statusCode, setting); ok {
			http.Redirect(rec, r, redirectURL, http.StatusFound)
			return
		}
		http.Error(rec, err.Error(), statusCode)
		return
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
		Auth:       principal.authData,
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
	statusCode = httpResp.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	rec.WriteHeader(statusCode)
	rec.Write([]byte(httpResp.Body))
}

func (h *HTTPTriggerHandler) resolveSetting(ctx context.Context, pluginID, triggerName string) (HTTPTriggerSetting, error) {
	setting := HTTPTriggerSetting{
		Enabled:          true,
		AllowUserKeys:    true,
		AllowServiceKeys: false,
	}
	if h.loadSetting == nil {
		return setting, nil
	}
	loaded, found, err := h.loadSetting(ctx, pluginID, triggerName)
	if err != nil {
		return HTTPTriggerSetting{}, err
	}
	if found {
		return loaded, nil
	}
	return setting, nil
}

func (h *HTTPTriggerHandler) resolvePrincipal(r *http.Request, pluginID, triggerName string, setting HTTPTriggerSetting) (resolvedHTTPPrincipal, int, error) {
	if token, ok := bearerToken(r); ok {
		return h.resolveBearerPrincipal(r.Context(), token, pluginID, triggerName, setting)
	}

	if h.authenticateUser != nil {
		if userID, ok := h.authenticateUser(r); ok {
			return h.authorizeUserPrincipal(r.Context(), pluginID, triggerName, setting, userID)
		}
	}

	return resolvedHTTPPrincipal{}, http.StatusUnauthorized, fmt.Errorf("authentication required")
}

func (h *HTTPTriggerHandler) resolveBearerPrincipal(ctx context.Context, rawToken, pluginID, triggerName string, setting HTTPTriggerSetting) (resolvedHTTPPrincipal, int, error) {
	if h.authenticateUserToken != nil {
		userID, ok, err := h.authenticateUserToken(ctx, rawToken)
		if err != nil {
			return resolvedHTTPPrincipal{}, http.StatusInternalServerError, fmt.Errorf("internal error")
		}
		if ok {
			return h.authorizeUserPrincipal(ctx, pluginID, triggerName, setting, userID)
		}
	}

	if h.authenticateService == nil {
		return resolvedHTTPPrincipal{}, http.StatusUnauthorized, fmt.Errorf("authentication required")
	}
	principal, ok, err := h.authenticateService(ctx, rawToken, pluginID, triggerName)
	if err != nil {
		return resolvedHTTPPrincipal{}, http.StatusInternalServerError, fmt.Errorf("internal error")
	}
	if !ok {
		return resolvedHTTPPrincipal{}, http.StatusUnauthorized, fmt.Errorf("authentication required")
	}
	if !setting.AllowServiceKeys {
		return resolvedHTTPPrincipal{}, http.StatusForbidden, fmt.Errorf("forbidden")
	}
	return resolvedHTTPPrincipal{
		authData: &model.HTTPAuthData{
			Kind:         model.HTTPAuthService,
			ServiceKeyID: principal.ID,
		},
	}, 0, nil
}

func (h *HTTPTriggerHandler) authorizeUserPrincipal(ctx context.Context, pluginID, triggerName string, setting HTTPTriggerSetting, userID model.GlobalUserID) (resolvedHTTPPrincipal, int, error) {
	if !setting.AllowUserKeys {
		return resolvedHTTPPrincipal{}, http.StatusForbidden, fmt.Errorf("forbidden")
	}
	if setting.PolicyExpression != "" {
		if h.evalPolicy == nil {
			return resolvedHTTPPrincipal{}, http.StatusInternalServerError, fmt.Errorf("authorization unavailable")
		}
		allowed, err := h.evalPolicy(ctx, setting.PolicyExpression, userID)
		if err != nil {
			slog.Warn("HTTP trigger policy expression error",
				"plugin", pluginID,
				"trigger", triggerName,
				"error", err)
			return resolvedHTTPPrincipal{}, http.StatusForbidden, fmt.Errorf("forbidden")
		}
		if !allowed {
			return resolvedHTTPPrincipal{}, http.StatusForbidden, fmt.Errorf("forbidden")
		}
	}
	return resolvedHTTPPrincipal{
		authData: &model.HTTPAuthData{
			Kind:   model.HTTPAuthUser,
			UserID: userID,
		},
	}, 0, nil
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

func loginRedirectURL(r *http.Request, statusCode int, setting HTTPTriggerSetting) (string, bool) {
	if statusCode != http.StatusUnauthorized {
		return "", false
	}
	if !setting.AllowUserKeys {
		return "", false
	}
	if _, ok := bearerToken(r); ok {
		return "", false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return "", false
	}
	if !acceptsHTML(r) {
		return "", false
	}

	return "/api/auth/tsu/start?return_to=" + url.QueryEscape(requestReturnTo(r)), true
}

func acceptsHTML(r *http.Request) bool {
	for _, value := range r.Header.Values("Accept") {
		for _, part := range strings.Split(value, ",") {
			if mediaType := strings.TrimSpace(strings.SplitN(part, ";", 2)[0]); mediaType == "text/html" {
				return true
			}
		}
	}
	return false
}

func requestReturnTo(r *http.Request) string {
	if r.URL == nil {
		return "/"
	}
	if raw := r.URL.RequestURI(); raw != "" {
		return raw
	}
	if r.URL.Path == "" {
		return "/"
	}
	return r.URL.Path
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// Set version 4 (random) and variant bits per RFC 4122.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
