//go:build !wasip1

package wasmplugin

import "fmt"

// CallPluginInto is only available when the plugin is built for WASI.
func CallPluginInto(target, method string, params interface{}, out interface{}) error {
	return fmt.Errorf("CallPluginInto is only available when targeting wasip1")
}
