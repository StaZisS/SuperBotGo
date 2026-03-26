package hostapi

import (
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestStripWirePrefix_WithPrefix(t *testing.T) {
	data := append([]byte{wirePrefix}, 0x82, 0xa3) // some msgpack bytes
	payload := stripWirePrefix(data)
	if len(payload) != 2 {
		t.Fatalf("expected 2 payload bytes, got %d", len(payload))
	}
}

func TestStripWirePrefix_WithoutPrefix(t *testing.T) {
	data := []byte{0x82, 0xa3}
	payload := stripWirePrefix(data)
	if len(payload) != 2 {
		t.Fatalf("expected 2 payload bytes, got %d", len(payload))
	}
}

func TestStripWirePrefix_Empty(t *testing.T) {
	payload := stripWirePrefix(nil)
	if payload != nil {
		t.Fatalf("expected nil payload, got %v", payload)
	}
}

func TestMarshalWire_HasPrefix(t *testing.T) {
	input := map[string]string{"a": "b"}
	data, err := marshalWire(input)
	if err != nil {
		t.Fatal(err)
	}
	if data[0] != wirePrefix {
		t.Fatalf("expected prefix 0x%02x, got 0x%02x", wirePrefix, data[0])
	}
}

func TestMarshalWire_RoundTrip(t *testing.T) {
	input := map[string]string{"hello": "world"}
	data, err := marshalWire(input)
	if err != nil {
		t.Fatal(err)
	}

	payload := stripWirePrefix(data)
	var output map[string]string
	if err := msgpack.Unmarshal(payload, &output); err != nil {
		t.Fatal(err)
	}
	if output["hello"] != "world" {
		t.Fatalf("expected world, got %q", output["hello"])
	}
}

func TestUnmarshalWire_RoundTrip(t *testing.T) {
	input := map[string]string{"hello": "world"}
	raw, err := msgpack.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	var output map[string]string
	if err := unmarshalWire(raw, &output); err != nil {
		t.Fatal(err)
	}
	if output["hello"] != "world" {
		t.Fatalf("expected world, got %q", output["hello"])
	}
}
