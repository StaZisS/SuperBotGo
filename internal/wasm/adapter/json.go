package adapter

import "encoding/json"

func cloneRawMessage(v json.RawMessage) json.RawMessage {
	if len(v) == 0 {
		return nil
	}
	out := make(json.RawMessage, len(v))
	copy(out, v)
	return out
}
