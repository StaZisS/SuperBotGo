package hostapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/tetratelabs/wazero/api"
	"github.com/vmihailenco/msgpack/v5"
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
	Requirement string            `msgpack:"requirement,omitempty"`
	Method      string            `msgpack:"method"`
	URL         string            `msgpack:"url"`
	Headers     map[string]string `msgpack:"headers,omitempty"`
	Body        string            `msgpack:"body,omitempty"`
}

type httpResponsePayload struct {
	StatusCode int               `msgpack:"status_code"`
	Headers    map[string]string `msgpack:"headers,omitempty"`
	Body       string            `msgpack:"body"`
}

func (h *HostAPI) httpRequestFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)

		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var payload httpRequestPayload
		if err := msgpack.Unmarshal(data, &payload); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if err := h.perms.CheckPermission(pluginID, "network"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.HTTP == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("HTTP"))
			return
		}

		method := normalizeHTTPRequestMethod(payload.Method)
		payload.Method = method
		policy := HTTPPolicy{}

		if h.httpPolicyEnabled {
			policy, err = h.HTTPPolicy(pluginID, payload.Requirement)
			if err != nil {
				returnError(ctx, mod, stack, err)
				return
			}
			if err := enforceHTTPRequestPolicy(policy, method, payload.URL, int64(len(payload.Body))); err != nil {
				returnError(ctx, mod, stack, err)
				return
			}
		}

		if isBlockedHost(payload.URL) {
			returnError(ctx, mod, stack, fmt.Errorf("requests to internal/private addresses are not allowed"))
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmHTTPMaxTimeout))
		defer cancel()

		var body io.Reader
		if payload.Body != "" {
			body = bytes.NewBufferString(payload.Body)
		}

		req, err := http.NewRequestWithContext(reqCtx, method, payload.URL, body)
		if err != nil {
			returnError(ctx, mod, stack, fmt.Errorf("create request: %w", err))
			return
		}

		for k, v := range payload.Headers {
			req.Header.Set(k, v)
		}

		resp, err := h.deps.HTTP.Do(req)
		if err != nil {
			returnError(ctx, mod, stack, fmt.Errorf("http request: %w", err))
			return
		}
		defer resp.Body.Close()

		responseLimit := defaultHTTPResponseLimit
		if h.httpPolicyEnabled {
			responseLimit = responseBodyReadLimit(policy)
		}

		respBody, err := readHTTPResponseBody(resp.Body, responseLimit)
		if err != nil {
			returnError(ctx, mod, stack, fmt.Errorf("http response body: %w", err))
			return
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

		writeResult(ctx, mod, stack, result)
	}
}
