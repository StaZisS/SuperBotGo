//go:build !wasip1

package wasmplugin

import (
	"strings"
	"testing"
)

func TestCallPluginIntoHostStub(t *testing.T) {
	err := CallPluginInto("demo", "ping", map[string]string{"ok": "true"}, nil)
	if err == nil {
		t.Fatal("CallPluginInto() error = nil, want stub error")
	}
	if !strings.Contains(err.Error(), "wasip1") {
		t.Fatalf("CallPluginInto() error = %q, want mention of wasip1", err.Error())
	}
}
