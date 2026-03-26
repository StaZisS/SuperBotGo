//go:build wasip1

package wasmplugin

import "github.com/vmihailenco/msgpack/v5"

const wirePrefix byte = 0x01

// marshalMsgpack serializes v to MessagePack with the wire prefix byte.
func marshalMsgpack(v any) ([]byte, error) {
	payload, err := msgpack.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 1+len(payload))
	out[0] = wirePrefix
	copy(out[1:], payload)
	return out, nil
}
