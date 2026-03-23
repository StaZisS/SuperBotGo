package hostapi

import (
	"testing"
)

func TestDetectEncoding_ExplicitJSON(t *testing.T) {
	data := append([]byte{byte(EncodingJSON)}, []byte(`{"key":"val"}`)...)
	enc, payload := detectEncoding(data)
	if enc != EncodingJSON {
		t.Fatalf("expected EncodingJSON, got %d", enc)
	}
	if string(payload) != `{"key":"val"}` {
		t.Fatalf("unexpected payload: %q", payload)
	}
}

func TestDetectEncoding_ExplicitMsgpack(t *testing.T) {
	data := append([]byte{byte(EncodingMsgpack)}, 0x82, 0xa3) // some msgpack bytes
	enc, payload := detectEncoding(data)
	if enc != EncodingMsgpack {
		t.Fatalf("expected EncodingMsgpack, got %d", enc)
	}
	if len(payload) != 2 {
		t.Fatalf("expected 2 payload bytes, got %d", len(payload))
	}
}

func TestDetectEncoding_LegacyJSON(t *testing.T) {
	// Raw JSON without prefix (backward compatible).
	data := []byte(`{"key":"val"}`)
	enc, payload := detectEncoding(data)
	if enc != EncodingJSON {
		t.Fatalf("expected EncodingJSON for legacy JSON, got %d", enc)
	}
	if string(payload) != `{"key":"val"}` {
		t.Fatalf("unexpected payload: %q", payload)
	}
}

func TestDetectEncoding_Empty(t *testing.T) {
	enc, payload := detectEncoding(nil)
	if enc != EncodingJSON {
		t.Fatalf("expected EncodingJSON for empty data, got %d", enc)
	}
	if payload != nil {
		t.Fatalf("expected nil payload, got %v", payload)
	}
}

func TestCodecFor(t *testing.T) {
	codec, err := CodecFor(EncodingJSON)
	if err != nil {
		t.Fatal(err)
	}
	if codec.Type() != EncodingJSON {
		t.Fatalf("expected JSON codec")
	}

	codec, err = CodecFor(EncodingMsgpack)
	if err != nil {
		t.Fatal(err)
	}
	if codec.Type() != EncodingMsgpack {
		t.Fatalf("expected Msgpack codec")
	}

	_, err = CodecFor(0xFF)
	if err == nil {
		t.Fatal("expected error for unknown encoding")
	}
}

func TestJSONCodecRoundTrip(t *testing.T) {
	codec := jsonCodec{}
	input := map[string]string{"hello": "world"}
	data, err := codec.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output map[string]string
	if err := codec.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output["hello"] != "world" {
		t.Fatalf("expected world, got %q", output["hello"])
	}
}

func TestMsgpackCodecRoundTrip(t *testing.T) {
	codec := msgpackCodec{}
	input := map[string]string{"hello": "world"}
	data, err := codec.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output map[string]string
	if err := codec.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output["hello"] != "world" {
		t.Fatalf("expected world, got %q", output["hello"])
	}
}

func TestPrefixedEncode(t *testing.T) {
	input := map[string]string{"a": "b"}

	// JSON prefixed encode.
	jsonData, err := prefixedEncode(codecJSON, input)
	if err != nil {
		t.Fatal(err)
	}
	if jsonData[0] != byte(EncodingJSON) {
		t.Fatalf("expected JSON prefix 0x00, got 0x%02x", jsonData[0])
	}
	// Verify JSON payload after prefix.
	enc, payload := detectEncoding(jsonData)
	if enc != EncodingJSON {
		t.Fatalf("expected JSON detection")
	}
	if string(payload) != `{"a":"b"}` {
		t.Fatalf("unexpected JSON payload: %q", payload)
	}

	// Msgpack prefixed encode.
	mpData, err := prefixedEncode(codecMsgpack, input)
	if err != nil {
		t.Fatal(err)
	}
	if mpData[0] != byte(EncodingMsgpack) {
		t.Fatalf("expected Msgpack prefix 0x01, got 0x%02x", mpData[0])
	}
	enc, payload = detectEncoding(mpData)
	if enc != EncodingMsgpack {
		t.Fatalf("expected Msgpack detection")
	}
	var output map[string]string
	if err := codecMsgpack.Unmarshal(payload, &output); err != nil {
		t.Fatal(err)
	}
	if output["a"] != "b" {
		t.Fatalf("expected b, got %q", output["a"])
	}
}

func TestMsgpackSmallerThanJSON(t *testing.T) {
	// Verify that msgpack is generally smaller for structured data.
	input := map[string]interface{}{
		"name":  "test-plugin",
		"count": 42,
		"items": []string{"alpha", "beta", "gamma"},
	}
	jsonData, _ := codecJSON.Marshal(input)
	mpData, _ := codecMsgpack.Marshal(input)

	if len(mpData) >= len(jsonData) {
		t.Logf("JSON size: %d, Msgpack size: %d", len(jsonData), len(mpData))
		t.Fatalf("expected msgpack to be smaller than JSON for this input")
	}
}
