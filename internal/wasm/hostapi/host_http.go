package hostapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tetratelabs/wazero/api"
)

const wasmHTTPTimeout = 30 * time.Second

func isBlockedHost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return true
	}

	hostname := u.Hostname()
	if hostname == "" {
		return true
	}

	lower := strings.ToLower(hostname)
	if lower == "localhost" || lower == "metadata.google.internal" {
		return true
	}

	if lower == "169.254.169.254" || lower == "metadata" {
		return true
	}

	ip := net.ParseIP(hostname)
	if ip == nil {
		return false
	}

	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

type httpRequestPayload struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

type httpResponsePayload struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body"`
}

func (h *HostAPI) httpRequestFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readModMemory(mod, offset, length)
		if err != nil {
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		var payload httpRequestPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		requiredPerm := "network:read"
		if payload.Method != "" && payload.Method != http.MethodGet && payload.Method != http.MethodHead {
			requiredPerm = "network:write"
		}
		if err := h.perms.CheckPermission(pluginID, requiredPerm); err != nil {
			writeErrorResult(ctx, mod, stack, err)
			return
		}

		if h.deps.HTTP == nil {
			writeErrorResult(ctx, mod, stack, errDepNotAvailable("HTTP"))
			return
		}

		if isBlockedHost(payload.URL) {
			writeErrorResult(ctx, mod, stack, fmt.Errorf("requests to internal/private addresses are not allowed"))
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, wasmHTTPTimeout)
		defer cancel()

		method := payload.Method
		if method == "" {
			method = http.MethodGet
		}

		var body io.Reader
		if payload.Body != "" {
			body = bytes.NewBufferString(payload.Body)
		}

		req, err := http.NewRequestWithContext(reqCtx, method, payload.URL, body)
		if err != nil {
			writeErrorResult(ctx, mod, stack, fmt.Errorf("create request: %w", err))
			return
		}

		for k, v := range payload.Headers {
			req.Header.Set(k, v)
		}

		resp, err := h.deps.HTTP.Do(req)
		if err != nil {
			writeErrorResult(ctx, mod, stack, fmt.Errorf("http request: %w", err))
			return
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			slog.Warn("wasm: http response body read truncated", "plugin", pluginID, "error", err)
		}

		headers := make(map[string]string)
		for k := range resp.Header {
			headers[k] = resp.Header.Get(k)
		}

		result := httpResponsePayload{
			StatusCode: resp.StatusCode,
			Headers:    headers,
			Body:       string(respBody),
		}

		writeJSONResult(ctx, mod, stack, result)
	}
}
