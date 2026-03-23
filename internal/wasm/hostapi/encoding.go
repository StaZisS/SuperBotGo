package hostapi

import (
	"encoding/json"
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

type EncodingType byte

const (
	EncodingJSON    EncodingType = 0x00
	EncodingMsgpack EncodingType = 0x01
)

type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
	Type() EncodingType
}

type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
func (jsonCodec) Type() EncodingType                 { return EncodingJSON }

type msgpackCodec struct{}

func (msgpackCodec) Marshal(v any) ([]byte, error)      { return msgpack.Marshal(v) }
func (msgpackCodec) Unmarshal(data []byte, v any) error { return msgpack.Unmarshal(data, v) }
func (msgpackCodec) Type() EncodingType                 { return EncodingMsgpack }

var (
	codecJSON    Codec = jsonCodec{}
	codecMsgpack Codec = msgpackCodec{}
)

func CodecFor(t EncodingType) (Codec, error) {
	switch t {
	case EncodingJSON:
		return codecJSON, nil
	case EncodingMsgpack:
		return codecMsgpack, nil
	default:
		return nil, fmt.Errorf("unknown encoding type: 0x%02x", t)
	}
}

func detectEncoding(data []byte) (EncodingType, []byte) {
	if len(data) == 0 {
		return EncodingJSON, data
	}

	switch data[0] {
	case byte(EncodingJSON):
		return EncodingJSON, data[1:]
	case byte(EncodingMsgpack):
		return EncodingMsgpack, data[1:]
	default:
		return EncodingJSON, data
	}
}

func prefixedEncode(codec Codec, v any) ([]byte, error) {
	payload, err := codec.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 1+len(payload))
	out[0] = byte(codec.Type())
	copy(out[1:], payload)
	return out, nil
}
