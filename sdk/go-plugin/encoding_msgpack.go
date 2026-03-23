//go:build wasip1

package wasmplugin

import "github.com/vmihailenco/msgpack/v5"

// marshalMsgpack serializes v to MessagePack with the encoding prefix byte.
func marshalMsgpack(v any) ([]byte, error) {
	payload, err := msgpack.Marshal(v)
	if err != nil {
		return nil, err
	}
	// Prepend the msgpack encoding type byte (0x01).
	out := make([]byte, 1+len(payload))
	out[0] = byte(encodingMsgpack)
	copy(out[1:], payload)
	return out, nil
}

// unmarshalMsgpackPayload deserializes a MessagePack payload (prefix already
// stripped) into v.
func unmarshalMsgpackPayload(data []byte, v any) error {
	return msgpack.Unmarshal(data, v)
}
