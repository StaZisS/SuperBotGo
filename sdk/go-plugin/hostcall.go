//go:build wasip1

package wasmplugin

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// ---------------------------------------------------------------------------
// WASM imports — host functions provided by the SuperBotGo runtime via the
// "env" host module. Each returns a packed i64: (offset << 32 | length).
// ---------------------------------------------------------------------------

//go:wasmimport env http_request
func _httpRequest(ptr uint32, length uint32) uint64

//go:wasmimport env call_plugin
func _callPlugin(ptr uint32, length uint32) uint64

//go:wasmimport env publish_event
func _publishEvent(ptr uint32, length uint32) uint64

// ---------------------------------------------------------------------------
// Encoding support
// ---------------------------------------------------------------------------

// hostEncoding controls the wire encoding used for host function calls.
// Default is JSON for backward compatibility. Set to EncodingMsgpack via
// UseMessagePack() for better performance on hot paths.
var hostEncoding encodingType = encodingJSON

type encodingType byte

const (
	encodingJSON    encodingType = 0x00
	encodingMsgpack encodingType = 0x01
)

// UseMessagePack switches host function calls to use MessagePack encoding.
// Call this from init() or at the start of main() before any host calls.
// The host will auto-detect the encoding and respond in the same format.
func UseMessagePack() {
	hostEncoding = encodingMsgpack
}

// UseJSON switches host function calls back to JSON encoding (the default).
func UseJSON() {
	hostEncoding = encodingJSON
}

// marshalForHost serializes v using the configured encoding and prepends the
// encoding prefix byte.
func marshalForHost(v any) ([]byte, error) {
	switch hostEncoding {
	case encodingMsgpack:
		return marshalMsgpack(v)
	default:
		// JSON: no prefix needed for backward compatibility with older hosts.
		return json.Marshal(v)
	}
}

// unmarshalFromHost deserializes response data from the host. It auto-detects
// the encoding by examining the first byte.
func unmarshalFromHost(data []byte, v any) error {
	if len(data) == 0 {
		return fmt.Errorf("empty response from host")
	}
	switch data[0] {
	case byte(encodingMsgpack):
		return unmarshalMsgpackPayload(data[1:], v)
	case byte(encodingJSON):
		// Explicit JSON prefix — strip it.
		return json.Unmarshal(data[1:], v)
	default:
		// Legacy raw JSON (starts with '{', '[', '"', etc.).
		return json.Unmarshal(data, v)
	}
}

// ---------------------------------------------------------------------------
// Host call helpers
// ---------------------------------------------------------------------------

// callHost marshals the payload, writes it to WASM memory, calls the host
// function, and reads + unmarshals the response.
func callHost(hostFn func(uint32, uint32) uint64, payload any) ([]byte, error) {
	data, err := marshalForHost(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal host call payload: %w", err)
	}

	ptr, sz := bytesToPtr(data)
	result := hostFn(ptr, sz)

	if result == 0 {
		return nil, nil // success with no response data
	}

	offset := uint32(result >> 32)
	length := uint32(result & 0xFFFFFFFF)

	return ptrToBytes(offset, length), nil
}

// callHostWithResult is like callHost but also unmarshals the response into v.
func callHostWithResult(hostFn func(uint32, uint32) uint64, payload any, v any) error {
	raw, err := callHost(hostFn, payload)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}
	// Check for error response.
	var errResp struct {
		Error string `json:"error" msgpack:"error"`
	}
	// Try to unmarshal as error first with a copy.
	errData := make([]byte, len(raw))
	copy(errData, raw)
	if unmarshalFromHost(errData, &errResp) == nil && errResp.Error != "" {
		return fmt.Errorf("host: %s", errResp.Error)
	}
	if v == nil {
		return nil
	}
	return unmarshalFromHost(raw, v)
}

// ---------------------------------------------------------------------------
// Public host function wrappers
// ---------------------------------------------------------------------------

// httpRequestPayload is the request payload for http_request.
type httpRequestPayload struct {
	Method  string            `json:"method" msgpack:"method"`
	URL     string            `json:"url" msgpack:"url"`
	Headers map[string]string `json:"headers,omitempty" msgpack:"headers,omitempty"`
	Body    string            `json:"body,omitempty" msgpack:"body,omitempty"`
}

// HTTPResponse is the response from an HTTP request.
type HTTPResponse struct {
	StatusCode int               `json:"status_code" msgpack:"status_code"`
	Headers    map[string]string `json:"headers,omitempty" msgpack:"headers,omitempty"`
	Body       string            `json:"body" msgpack:"body"`
}

// HTTPRequest makes an HTTP request through the host.
func HTTPRequest(method, url string, headers map[string]string, body string) (*HTTPResponse, error) {
	payload := httpRequestPayload{
		Method:  method,
		URL:     url,
		Headers: headers,
		Body:    body,
	}
	var resp HTTPResponse
	if err := callHostWithResult(_httpRequest, payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// HTTPGet is a convenience wrapper for GET requests.
func HTTPGet(url string) (*HTTPResponse, error) {
	return HTTPRequest("GET", url, nil, "")
}

// HTTPPost is a convenience wrapper for POST requests.
func HTTPPost(url string, contentType string, body string) (*HTTPResponse, error) {
	return HTTPRequest("POST", url, map[string]string{"Content-Type": contentType}, body)
}

// callPluginPayload is the request payload for call_plugin.
type callPluginPayload struct {
	Target string          `json:"target" msgpack:"target"`
	Method string          `json:"method" msgpack:"method"`
	Params json.RawMessage `json:"params,omitempty" msgpack:"params,omitempty"`
}

// CallPlugin calls another plugin through the host.
func CallPlugin(target, method string, params interface{}) ([]byte, error) {
	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal call_plugin params: %w", err)
		}
	}
	payload := callPluginPayload{
		Target: target,
		Method: method,
		Params: rawParams,
	}
	return callHost(_callPlugin, payload)
}

// publishEventPayload is the request payload for publish_event.
type publishEventPayload struct {
	Topic   string          `json:"topic" msgpack:"topic"`
	Payload json.RawMessage `json:"payload" msgpack:"payload"`
}

// PublishEvent publishes an event through the host event bus.
func PublishEvent(topic string, payload interface{}) error {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	p := publishEventPayload{
		Topic:   topic,
		Payload: json.RawMessage(rawPayload),
	}
	raw, err := callHost(_publishEvent, p)
	if err != nil {
		return err
	}
	if raw != nil {
		var errResp struct {
			Error string `json:"error" msgpack:"error"`
		}
		if unmarshalFromHost(raw, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("host: %s", errResp.Error)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// ptrLen packs a pointer and length for calling host functions.
func ptrLen(b []byte) (uint32, uint32) {
	if len(b) == 0 {
		return 0, 0
	}
	return uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b))
}
