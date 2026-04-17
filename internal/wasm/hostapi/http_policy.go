package hostapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"

	wasmrt "SuperBotGo/internal/wasm/runtime"
)

const (
	defaultHTTPRequirementName = "default"
	defaultHTTPResponseLimit   = int64(1 << 20)
)

type HTTPPolicy struct {
	Requirement          string
	AllowedHosts         []string
	AllowedMethods       []string
	MaxRequestBodyBytes  int64
	MaxResponseBodyBytes int64
}

type httpPolicyConfig struct {
	AllowedHosts         []string `json:"allowed_hosts"`
	AllowedMethods       []string `json:"allowed_methods"`
	MaxRequestBodyBytes  int64    `json:"max_request_body_bytes"`
	MaxResponseBodyBytes int64    `json:"max_response_body_bytes"`
}

type pluginHTTPConfig struct {
	Requirements struct {
		HTTP map[string]httpPolicyConfig `json:"http"`
	} `json:"requirements"`
}

type httpPolicyStore struct {
	mu       sync.RWMutex
	policies map[string]map[string]HTTPPolicy
}

func newHTTPPolicyStore() *httpPolicyStore {
	return &httpPolicyStore{
		policies: make(map[string]map[string]HTTPPolicy),
	}
}

func (s *httpPolicyStore) Set(pluginID string, policies map[string]HTTPPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(policies) == 0 {
		delete(s.policies, pluginID)
		return
	}
	cloned := make(map[string]HTTPPolicy, len(policies))
	for name, policy := range policies {
		cloned[name] = cloneHTTPPolicy(policy)
	}
	s.policies[pluginID] = cloned
}

func (s *httpPolicyStore) Delete(pluginID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.policies, pluginID)
}

func (s *httpPolicyStore) Get(pluginID, requirement string) (HTTPPolicy, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	perPlugin, ok := s.policies[pluginID]
	if !ok {
		return HTTPPolicy{}, false
	}
	policy, ok := perPlugin[requirement]
	if !ok {
		return HTTPPolicy{}, false
	}
	return cloneHTTPPolicy(policy), true
}

func cloneHTTPPolicy(policy HTTPPolicy) HTTPPolicy {
	policy.AllowedHosts = append([]string(nil), policy.AllowedHosts...)
	policy.AllowedMethods = append([]string(nil), policy.AllowedMethods...)
	return policy
}

func ResolveHTTPPolicies(requirements []wasmrt.RequirementDef, config json.RawMessage) (map[string]HTTPPolicy, error) {
	policies := make(map[string]HTTPPolicy)

	var cfg pluginHTTPConfig
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, fmt.Errorf("parse http requirement config: %w", err)
		}
	}

	for _, req := range requirements {
		if req.Type != "http" {
			continue
		}

		name := req.Name
		if name == "" {
			name = defaultHTTPRequirementName
		}

		policy := HTTPPolicy{Requirement: name}
		if raw, ok := cfg.Requirements.HTTP[name]; ok {
			policy.AllowedHosts = normalizeAllowedHosts(raw.AllowedHosts)
			policy.AllowedMethods = normalizeAllowedMethods(raw.AllowedMethods)
			if raw.MaxRequestBodyBytes < 0 {
				return nil, fmt.Errorf("http requirement %q has negative max_request_body_bytes", name)
			}
			if raw.MaxResponseBodyBytes < 0 {
				return nil, fmt.Errorf("http requirement %q has negative max_response_body_bytes", name)
			}
			policy.MaxRequestBodyBytes = raw.MaxRequestBodyBytes
			policy.MaxResponseBodyBytes = raw.MaxResponseBodyBytes
		}

		policies[name] = policy
	}

	return policies, nil
}

func normalizeHTTPRequestMethod(method string) string {
	method = strings.TrimSpace(method)
	if method == "" {
		return "GET"
	}
	return strings.ToUpper(method)
}

func normalizeAllowedMethods(methods []string) []string {
	if len(methods) == 0 {
		return nil
	}
	result := make([]string, 0, len(methods))
	seen := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		normalized := normalizeHTTPRequestMethod(method)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func normalizeAllowedHosts(hosts []string) []string {
	if len(hosts) == 0 {
		return nil
	}
	result := make([]string, 0, len(hosts))
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		normalized := strings.ToLower(strings.TrimSpace(host))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func (h *HostAPI) SetHTTPPolicyEnforcement(enabled bool) {
	h.httpPolicyEnabled = enabled
}

func (h *HostAPI) SetHTTPPolicies(pluginID string, policies map[string]HTTPPolicy) {
	h.httpPolicies.Set(pluginID, policies)
}

func (h *HostAPI) HTTPPolicy(pluginID, requirement string) (HTTPPolicy, error) {
	if requirement == "" {
		requirement = defaultHTTPRequirementName
	}
	policy, ok := h.httpPolicies.Get(pluginID, requirement)
	if !ok {
		return HTTPPolicy{}, fmt.Errorf("plugin %q has no http requirement %q", pluginID, requirement)
	}
	return policy, nil
}

func enforceHTTPRequestPolicy(policy HTTPPolicy, method, rawURL string, requestBodySize int64) error {
	method = normalizeHTTPRequestMethod(method)

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse request url: %w", err)
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return fmt.Errorf("request url has empty host")
	}

	if len(policy.AllowedHosts) > 0 && !hostAllowedByPolicy(host, policy.AllowedHosts) {
		return fmt.Errorf("http requirement %q does not allow host %q", policy.Requirement, host)
	}
	if len(policy.AllowedMethods) > 0 && !methodAllowedByPolicy(method, policy.AllowedMethods) {
		return fmt.Errorf("http requirement %q does not allow method %q", policy.Requirement, method)
	}
	if policy.MaxRequestBodyBytes > 0 && requestBodySize > policy.MaxRequestBodyBytes {
		return fmt.Errorf("http request body exceeds limit of %d bytes", policy.MaxRequestBodyBytes)
	}

	return nil
}

func methodAllowedByPolicy(method string, allowed []string) bool {
	for _, candidate := range allowed {
		if candidate == "*" || candidate == method {
			return true
		}
	}
	return false
}

func hostAllowedByPolicy(host string, allowed []string) bool {
	for _, candidate := range allowed {
		if candidate == host {
			return true
		}
		if strings.HasPrefix(candidate, "*.") {
			suffix := strings.TrimPrefix(candidate, "*")
			base := strings.TrimPrefix(candidate, "*.")
			if host != base && strings.HasSuffix(host, suffix) {
				return true
			}
		}
	}
	return false
}

func responseBodyReadLimit(policy HTTPPolicy) int64 {
	if policy.MaxResponseBodyBytes > 0 {
		return policy.MaxResponseBodyBytes
	}
	return defaultHTTPResponseLimit
}

func readHTTPResponseBody(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = defaultHTTPResponseLimit
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("http response body exceeds limit of %d bytes", limit)
	}
	return data, nil
}
