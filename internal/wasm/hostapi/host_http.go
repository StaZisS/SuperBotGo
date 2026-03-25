package hostapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/tetratelabs/wazero/api"
)

var wasmHTTPMaxTimeout = time.Duration(wasmrt.DefaultHostHTTPTimeoutSeconds) * time.Second

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
	Method  string            `json:"method" msgpack:"method"`
	URL     string            `json:"url" msgpack:"url"`
	Headers map[string]string `json:"headers,omitempty" msgpack:"headers,omitempty"`
	Body    string            `json:"body,omitempty" msgpack:"body,omitempty"`
}

type httpResponsePayload struct {
	StatusCode int               `json:"status_code" msgpack:"status_code"`
	Headers    map[string]string `json:"headers,omitempty" msgpack:"headers,omitempty"`
	Body       string            `json:"body" msgpack:"body"`
}

func (h *HostAPI) httpRequestFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)

		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var payload httpRequestPayload
		if err := unmarshalPayload(data, enc, &payload); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if err := h.perms.CheckPermission(pluginID, "network"); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.deps.HTTP == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("HTTP"), enc)
			return
		}

		if isBlockedHost(payload.URL) {
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("requests to internal/private addresses are not allowed"), enc)
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmHTTPMaxTimeout))
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
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("create request: %w", err), enc)
			return
		}

		for k, v := range payload.Headers {
			req.Header.Set(k, v)
		}

		resp, err := h.deps.HTTP.Do(req)
		if err != nil {
			returnErrorEnc(ctx, mod, stack, fmt.Errorf("http request: %w", err), enc)
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

		writeEncodedResult(ctx, mod, stack, result, enc)
	}
}
