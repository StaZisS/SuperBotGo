package wasmplugin

import "github.com/vmihailenco/msgpack/v5"

// RPCContext is passed to plugin RPC handlers.
type RPCContext struct {
	Caller string
	Method string
	Params []byte

	config map[string]interface{}
	logs   []logEntry
}

// Decode unmarshals RPC params from msgpack into v.
func (ctx *RPCContext) Decode(v interface{}) error {
	return msgpack.Unmarshal(ctx.Params, v)
}

// Config returns a config value by key, or fallback when absent.
func (ctx *RPCContext) Config(key, fallback string) string {
	if ctx.config == nil {
		return fallback
	}
	v, ok := ctx.config[key]
	if !ok {
		return fallback
	}
	s, ok := v.(string)
	if !ok {
		return fallback
	}
	return s
}

// Log records an informational log entry for the host.
func (ctx *RPCContext) Log(msg string) {
	ctx.logs = append(ctx.logs, logEntry{Level: "info", Msg: msg})
}

// LogError records an error log entry for the host.
func (ctx *RPCContext) LogError(msg string) {
	ctx.logs = append(ctx.logs, logEntry{Level: "error", Msg: msg})
}

// MarshalRPC marshals v as msgpack for RPC responses.
func MarshalRPC(v interface{}) ([]byte, error) {
	return msgpack.Marshal(v)
}

// CallPluginInto calls another plugin and unmarshals the msgpack result into out.
func CallPluginInto(target, method string, params interface{}, out interface{}) error {
	raw, err := CallPlugin(target, method, params)
	if err != nil {
		return err
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return msgpack.Unmarshal(stripPrefix(raw), out)
}
