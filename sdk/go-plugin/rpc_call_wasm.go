//go:build wasip1

package wasmplugin

import "github.com/vmihailenco/msgpack/v5"

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
