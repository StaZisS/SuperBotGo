//go:build wasip1

package wasmplugin

import (
	"encoding/json"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// KV host function imports (wasm -> host)
// ---------------------------------------------------------------------------

//go:wasmimport env kv_get
func _kv_get(offset, length uint32) uint64

//go:wasmimport env kv_set
func _kv_set(offset, length uint32) uint64

//go:wasmimport env kv_delete
func _kv_delete(offset, length uint32) uint64

//go:wasmimport env kv_list
func _kv_list(offset, length uint32) uint64

// ---------------------------------------------------------------------------
// Request / response types
// ---------------------------------------------------------------------------

type kvGetRequest struct {
	Key string `json:"key"`
}

type kvGetResponse struct {
	Value *string `json:"value"`
	Found bool    `json:"found"`
	Error string  `json:"error,omitempty"`
}

type kvSetRequest struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	TTLSeconds *int   `json:"ttl_seconds,omitempty"`
}

type kvSetResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type kvDeleteRequest struct {
	Key string `json:"key"`
}

type kvDeleteResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type kvListRequest struct {
	Prefix string `json:"prefix,omitempty"`
}

type kvListResponse struct {
	Keys  []string `json:"keys"`
	Error string   `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// Public API on EventContext
// ---------------------------------------------------------------------------

// KVGet retrieves a value from the plugin-scoped key-value store.
// Returns the value and true if found, or ("", false, nil) if the key does
// not exist. Returns a non-nil error only on host communication failures.
func (ctx *EventContext) KVGet(key string) (string, bool, error) {
	req, _ := json.Marshal(kvGetRequest{Key: key})
	ptr, length := bytesToPtr(req)

	packed := _kv_get(ptr, length)
	if packed == 0 {
		return "", false, fmt.Errorf("kv_get: host returned null")
	}

	respOffset := uint32(packed >> 32)
	respLength := uint32(packed & 0xFFFFFFFF)
	respData := ptrToBytes(respOffset, respLength)

	var resp kvGetResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return "", false, fmt.Errorf("kv_get: unmarshal response: %w", err)
	}
	if resp.Error != "" {
		return "", false, fmt.Errorf("kv_get: %s", resp.Error)
	}
	if !resp.Found || resp.Value == nil {
		return "", false, nil
	}
	return *resp.Value, true, nil
}

// KVSet stores a key-value pair in the plugin-scoped key-value store.
// The value persists in host memory across plugin executions (until the host
// restarts or the plugin is unloaded).
func (ctx *EventContext) KVSet(key, value string) error {
	req, _ := json.Marshal(kvSetRequest{Key: key, Value: value})
	ptr, length := bytesToPtr(req)

	packed := _kv_set(ptr, length)
	if packed == 0 {
		return fmt.Errorf("kv_set: host returned null")
	}

	respOffset := uint32(packed >> 32)
	respLength := uint32(packed & 0xFFFFFFFF)
	respData := ptrToBytes(respOffset, respLength)

	var resp kvSetResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return fmt.Errorf("kv_set: unmarshal response: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("kv_set: %s", resp.Error)
	}
	return nil
}

// KVSetWithTTL stores a key-value pair with a time-to-live. After the TTL
// expires the key is automatically removed from the store.
func (ctx *EventContext) KVSetWithTTL(key, value string, ttl time.Duration) error {
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		seconds = 1
	}
	req, _ := json.Marshal(kvSetRequest{Key: key, Value: value, TTLSeconds: &seconds})
	ptr, length := bytesToPtr(req)

	packed := _kv_set(ptr, length)
	if packed == 0 {
		return fmt.Errorf("kv_set: host returned null")
	}

	respOffset := uint32(packed >> 32)
	respLength := uint32(packed & 0xFFFFFFFF)
	respData := ptrToBytes(respOffset, respLength)

	var resp kvSetResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return fmt.Errorf("kv_set: unmarshal response: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("kv_set: %s", resp.Error)
	}
	return nil
}

// KVDelete removes a key from the plugin-scoped key-value store.
// It is a no-op if the key does not exist.
func (ctx *EventContext) KVDelete(key string) error {
	req, _ := json.Marshal(kvDeleteRequest{Key: key})
	ptr, length := bytesToPtr(req)

	packed := _kv_delete(ptr, length)
	if packed == 0 {
		return fmt.Errorf("kv_delete: host returned null")
	}

	respOffset := uint32(packed >> 32)
	respLength := uint32(packed & 0xFFFFFFFF)
	respData := ptrToBytes(respOffset, respLength)

	var resp kvDeleteResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return fmt.Errorf("kv_delete: unmarshal response: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("kv_delete: %s", resp.Error)
	}
	return nil
}

// KVList returns all keys in the plugin-scoped key-value store that match
// the given prefix. Pass an empty string to list all keys.
func (ctx *EventContext) KVList(prefix string) ([]string, error) {
	req, _ := json.Marshal(kvListRequest{Prefix: prefix})
	ptr, length := bytesToPtr(req)

	packed := _kv_list(ptr, length)
	if packed == 0 {
		return nil, fmt.Errorf("kv_list: host returned null")
	}

	respOffset := uint32(packed >> 32)
	respLength := uint32(packed & 0xFFFFFFFF)
	respData := ptrToBytes(respOffset, respLength)

	var resp kvListResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("kv_list: unmarshal response: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("kv_list: %s", resp.Error)
	}
	return resp.Keys, nil
}

// ---------------------------------------------------------------------------
// Public API on MigrateContext — mirrors EventContext KV methods so that
// migrate handlers can read/write/delete stored data during version upgrades.
// ---------------------------------------------------------------------------

// KVGet retrieves a value from the plugin-scoped key-value store.
func (ctx *MigrateContext) KVGet(key string) (string, bool, error) {
	req, _ := json.Marshal(kvGetRequest{Key: key})
	ptr, length := bytesToPtr(req)

	packed := _kv_get(ptr, length)
	if packed == 0 {
		return "", false, fmt.Errorf("kv_get: host returned null")
	}

	respOffset := uint32(packed >> 32)
	respLength := uint32(packed & 0xFFFFFFFF)
	respData := ptrToBytes(respOffset, respLength)

	var resp kvGetResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return "", false, fmt.Errorf("kv_get: unmarshal response: %w", err)
	}
	if resp.Error != "" {
		return "", false, fmt.Errorf("kv_get: %s", resp.Error)
	}
	if !resp.Found || resp.Value == nil {
		return "", false, nil
	}
	return *resp.Value, true, nil
}

// KVSet stores a key-value pair in the plugin-scoped key-value store.
func (ctx *MigrateContext) KVSet(key, value string) error {
	req, _ := json.Marshal(kvSetRequest{Key: key, Value: value})
	ptr, length := bytesToPtr(req)

	packed := _kv_set(ptr, length)
	if packed == 0 {
		return fmt.Errorf("kv_set: host returned null")
	}

	respOffset := uint32(packed >> 32)
	respLength := uint32(packed & 0xFFFFFFFF)
	respData := ptrToBytes(respOffset, respLength)

	var resp kvSetResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return fmt.Errorf("kv_set: unmarshal response: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("kv_set: %s", resp.Error)
	}
	return nil
}

// KVDelete removes a key from the plugin-scoped key-value store.
func (ctx *MigrateContext) KVDelete(key string) error {
	req, _ := json.Marshal(kvDeleteRequest{Key: key})
	ptr, length := bytesToPtr(req)

	packed := _kv_delete(ptr, length)
	if packed == 0 {
		return fmt.Errorf("kv_delete: host returned null")
	}

	respOffset := uint32(packed >> 32)
	respLength := uint32(packed & 0xFFFFFFFF)
	respData := ptrToBytes(respOffset, respLength)

	var resp kvDeleteResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return fmt.Errorf("kv_delete: unmarshal response: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("kv_delete: %s", resp.Error)
	}
	return nil
}

// KVList returns all keys matching the given prefix.
func (ctx *MigrateContext) KVList(prefix string) ([]string, error) {
	req, _ := json.Marshal(kvListRequest{Prefix: prefix})
	ptr, length := bytesToPtr(req)

	packed := _kv_list(ptr, length)
	if packed == 0 {
		return nil, fmt.Errorf("kv_list: host returned null")
	}

	respOffset := uint32(packed >> 32)
	respLength := uint32(packed & 0xFFFFFFFF)
	respData := ptrToBytes(respOffset, respLength)

	var resp kvListResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("kv_list: unmarshal response: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("kv_list: %s", resp.Error)
	}
	return resp.Keys, nil
}
