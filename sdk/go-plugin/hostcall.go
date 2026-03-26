//go:build wasip1

package wasmplugin

import (
	"fmt"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
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
// Host call helpers
// ---------------------------------------------------------------------------

// callHost marshals the payload, writes it to WASM memory, calls the host
// function, and reads + unmarshals the response.
func callHost(hostFn func(uint32, uint32) uint64, payload any) ([]byte, error) {
	data, err := marshalMsgpack(payload)
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

	data := stripPrefix(raw)

	// Check for error response.
	var errResp struct {
		Error string `msgpack:"error"`
	}
	errData := make([]byte, len(data))
	copy(errData, data)
	if msgpack.Unmarshal(errData, &errResp) == nil && errResp.Error != "" {
		return fmt.Errorf("host: %s", errResp.Error)
	}
	if v == nil {
		return nil
	}
	return msgpack.Unmarshal(data, v)
}

// stripPrefix removes the wire prefix byte (0x01) if present.
func stripPrefix(data []byte) []byte {
	if len(data) > 0 && data[0] == 0x01 {
		return data[1:]
	}
	return data
}

// ---------------------------------------------------------------------------
// Public host function wrappers
// ---------------------------------------------------------------------------

// httpRequestPayload is the request payload for http_request.
type httpRequestPayload struct {
	Method  string            `msgpack:"method"`
	URL     string            `msgpack:"url"`
	Headers map[string]string `msgpack:"headers,omitempty"`
	Body    string            `msgpack:"body,omitempty"`
}

// HTTPResponse is the response from an HTTP request.
type HTTPResponse struct {
	StatusCode int               `msgpack:"status_code"`
	Headers    map[string]string `msgpack:"headers,omitempty"`
	Body       string            `msgpack:"body"`
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
	Target string `msgpack:"target"`
	Method string `msgpack:"method"`
	Params []byte `msgpack:"params,omitempty"`
}

// CallPlugin calls another plugin through the host.
func CallPlugin(target, method string, params interface{}) ([]byte, error) {
	var rawParams []byte
	if params != nil {
		var err error
		rawParams, err = msgpack.Marshal(params)
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
	Topic   string `msgpack:"topic"`
	Payload []byte `msgpack:"payload"`
}

// PublishEvent publishes an event through the host event bus.
func PublishEvent(topic string, payload interface{}) error {
	rawPayload, err := msgpack.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	p := publishEventPayload{
		Topic:   topic,
		Payload: rawPayload,
	}
	raw, err := callHost(_publishEvent, p)
	if err != nil {
		return err
	}
	if raw != nil {
		data := stripPrefix(raw)
		var errResp struct {
			Error string `msgpack:"error"`
		}
		if msgpack.Unmarshal(data, &errResp) == nil && errResp.Error != "" {
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
