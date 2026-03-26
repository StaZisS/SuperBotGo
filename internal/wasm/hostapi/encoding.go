package hostapi

import "github.com/vmihailenco/msgpack/v5"

const wirePrefix byte = 0x01

func marshalWire(v any) ([]byte, error) {
	payload, err := msgpack.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 1+len(payload))
	out[0] = wirePrefix
	copy(out[1:], payload)
	return out, nil
}

func unmarshalWire(data []byte, v any) error {
	return msgpack.Unmarshal(data, v)
}

func stripWirePrefix(data []byte) []byte {
	if len(data) > 0 && data[0] == wirePrefix {
		return data[1:]
	}
	return data
}
